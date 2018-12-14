# Self-sharding caches in a cluster

An Ingress may be annotated so that the Varnish instances in a cluster
that implement the Ingress also implement sharding of their caches --
for each request, there is an instance in the cluster that "owns" the
potentially cached response, and other instances in the cluster
forward the request to that instance. We refer to this as
"self-sharding", because no sharding on the part of a component that
forwards requests to the cluster (such as a load balancer) is required
to shard the requests.  Requests may be distributed to the cluster in
any way, and the Varnish instances take care of the sharding.

The technical details for
[configuring self-sharding](#annotation-syntax-for-self-sharding) are given
further below. The next two sections describe in more detail how the
sharding feature affects a Varnish cluster.

## Clustering without sharding

To understand how the self-sharding feature works, first consider a
common design for clustering Varnish without sharding. This is in fact
common with or without Kubernetes, but Kubernetes terminology will be
used in the following.

![unsharded clustering](cluster-no-shard.png?raw=true "Unsharded Clustering")

A load balancer distributes requests uniformaly to Varnish instances
in the cluster, for example in round-robin order. Each Varnish
instance in turn distributes requests to backend Services, and
maintains its own cache for all cacheable responses from the Services.

Some of the effects of this design are:

* Each instance of Varnish accumulates approximately the same cache
  for cacheable responses in the system. The memory load for the cache
  is approximately duplicated by each instance.

* For N replicas in the Varnish cluster, cacheable responses are
  fetched N times from the Services that send them; and they are
  re-fetched N times when the TTL expires.

* Even after the response to a request has been cached, the next
  request for the same object may be a cache miss, if the LoadBalancer
  happens to forward the request to a Varnish instance that doesn't
  have it yet in its cache.

The purpose of self-sharding is to:

* distribute memory load for the cache among the instances in the
  cluster.

* reduce the request load on Services, so that there is only one fetch
  for a cacheable response, from one Varnish instance, until the TTL
  expires.

* ensure that if a cached response has been fetched from a Service
  just once, then any further request for the same object will be a
  cache hit until the TTL expires, regardless of which Varnish
  instance receives the request from the LoadBalancer.

## Clustering with sharding

When self-sharding is implemented, then for each request, there is a
Varnish instance in the cluster that forwards it to a Service. If the
response is cached, then that instance serves as the primary location
for the cached object. Each instance may forward a request to another
instance, or handle the request itself:

![Sharded clustering](cluster-sharded.png?raw=true "Sharded Clustering")

In the illustration, a request represented in red is received by a
Varnish instance and forwarded to another instance, which in turn
forwards it to a backend Service, or answers the request from cache.
Another request represented by green is handled by the instance itself
without forwarding. No special configuration for the LoadBalancer is
required -- it can, for example, continue distributing requests to
Varnishen in round-robin order.

When each instance is loaded with the same VCL configuration generated
by the Ingress controller, then they each forward requests in the same
way. When Varnish instances (Pods) are added to or removed from the
cluster, the controller reloads the resulting set of active instances
with the configuration for sharding among those instances.

Some features that result from self-sharding are:

* The Varnish instance that fetched a cached response serves as the
  "primary" location for the cached object. If a forwarding Varnish
  instance receives a cacheable response from another instance, it may
  also cache the response, but only for a limited time bounded by the
  ``max-secondary-ttl`` parameter described below. So the total memory
  load for the cache in the cluster is reduced.

* A cacheable response is fetched from a Service only once, from the
  instance that handles the request itself without forwarding. When
  the TTL expires, it is re-fetched only once from the same instance.

* If a cacheable response is in at least one the cluster's caches,
  then subsequent requests for the same object will be cache hits
  while the TTL is still valid, regardless of which Varnish instance
  received the request from the LoadBalancer.

* When instances are added to or removed from the cluster, the
  forwarding of requests to instances changes only as necessary. New
  instances receive requests that had been previously forwarded to
  other instances; when an instance is removed, the requests it had
  received are forwarded to other instances. But forwarding rules that
  are not affected by these changes remain the same -- they forward
  requests to the same instances, and hence may hit the same caches.

* The instances in the cluster are configured in VCL as backends for
  one another in this configuration, and as such they respond to
  health probes from one another. If an instance is found to be
  unhealthy, then each instance forwards a request to another instance
  in the same way; the new forwarding destination becomes the new
  "primary" location for cached objects, until the sick instance
  becomes healthy again. Some of the parameters of a Varnish health
  probe are configurable with the ``probe-*`` group of annotations
  described below.

## Annotation syntax for self-sharding

Self-sharding is configured by using the annotations
``ingress.varnish-cache.org/self-sharding`` in an Ingress, for
example:

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: cafe-ingress-varnish
  annotations:
    kubernetes.io/ingress.class: "varnish"
    ingress.varnish-cache.org/self-sharding: "on"
    ingress.varnish-cache.org/self-sharding-probe-timeout: "6s"
    ingress.varnish-cache.org/self-sharding-probe-interval: "6s"
    ingress.varnish-cache.org/self-sharding-probe-initial: "2"
    ingress.varnish-cache.org/self-sharding-probe-window: "4"
    ingress.varnish-cache.org/self-sharding-probe-threshold: "3"
    ingress.varnish-cache.org/self-sharding-max-secondary-ttl: "1m"
  namespace: varnish-ingress
# ...
```

VCL to implement self-sharding is generated by the controller for any
Varnish cluster that implements the Ingress.

Self-sharding is implemented when the
``ingress.varnish-cache.org/self-sharding`` annotation has the value
``on`` or ``true`` (case-insensitive). If the annotation is not
defined for an Ingress, or if it has any other value, then sharding is
not configured, and all other ``self-sharding-*`` annotations are
ignored.

These parameters configure the corresponding values for the
[health probes](https://varnish-cache.org/docs/6.1/reference/vcl.html#probes)
that instances in the cluster use to check one another's health:

* ``self-sharding-probe-timeout``
* ``self-sharding-probe-interval``
* ``self-sharding-probe-initial``
* ``self-sharding-probe-window``
* ``self-sharding-probe-threshold``

The default values are the Varnish defaults for probes (in Varnish
6.1.1).

The values for probes MUST be valid as values in the VCL source for
the probe; for example, the ``timeout`` and ``interval`` parameters
must be valid for the VCL
[DURATION type](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)
(examples are ``1s`` for one second, or ``1m`` for a
minute). Constraints on the values of ``initial``, ``window`` and
``threshold`` MUST also be satisfied; for example, they must all be >=
0, and ``threshold`` may not be larger than ``window``. Check the
documentation linked above for details.

The ``self-sharding-max-secondary-ttl`` parameter defaults to ``5m``
(5 minutes), and sets the upper bound for "secondary" caching as
discussed above. If an instance forwards a request to another instance
and the response is cacheable, then the forwarding instance may also
cache the response (as does the "primary" instance). But the secondary
instance will not retain the response for longer than the value of
``max-secondary-ttl`` (it is likely to hit the primary cache after
that TTL expires). Keeping this value low relative to the "primary"
TTLs serves to reduce the total memory load for caching in the
cluster.

As with the values for ``probe-timeout`` and ``probe-interval``, the
value of ``max-secondary-ttl`` MUST be a legal DURATION in VCL.

## Errors

As discussed above, the configuration values in the annotations for
self-sharding must be legal for the generated VCL (for example with
correct use of DURATION values). Otherwise, VCL will fail to load, and
the desired state for the Ingress containing the annotations will not
be achieved.

Because of the asynchronous nature of Kubernetes, the validity of the
annotations unfortunately cannot be checked when the manifest is
applied.  Errors only become known when the controller attempts to
load the generated VCL corresponding to the Ingress and the
self-sharding annotations, and Varnish responds with an error.

Check the log of the Ingress controller to verify successful load of a
configuration corresponding to the Ingress. If the load failed, the
log will contain error entries that include the VCL compiler error
message from Varnish.
