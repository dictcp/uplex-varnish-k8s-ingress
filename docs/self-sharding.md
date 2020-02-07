# Self-sharding caches in a cluster

An ``VarnishConfig`` resource may be configured so that the Varnish
instances in a cluster that implement Ingress also implement sharding
of their caches -- for each request, there is an instance in the
cluster that "owns" the potentially cached response, and other
instances in the cluster forward the request to that instance. We
refer to this as "self-sharding", because no sharding on the part of a
component that forwards requests to the cluster (such as a load
balancer) is required to shard the requests.  Requests may be
distributed to the cluster in any way, and the Varnish instances take
care of the sharding.

A sample manifest for the ``VarnishConfig`` Custom Resource to
configure self-sharding is in the
[``examples/``](/examples/self-sharding) folder.

The [technical reference](/docs/ref-varnish-cfg.md) for
``VarnishConfig`` specifies the configuration syntax for
self-sharding.  The next two sections describe in more detail how the
sharding feature affects a Varnish cluster.

## Clustering without sharding

To understand how the self-sharding feature works, first consider a
common design for clustering Varnish without sharding. This is in fact
common with or without Kubernetes, but Kubernetes terminology will be
used in the following.

![unsharded clustering](cluster-no-shard.png?raw=true "Unsharded Clustering")

A load balancer distributes requests uniformly to Varnish instances
in the cluster, for example in round-robin order. Each Varnish
instance in turn distributes requests to backend Services, and
maintains its own cache for all cacheable responses from the Services.

Some of the effects of this design are:

* Each instance of Varnish accumulates approximately the same cache
  for cacheable responses in the system. The memory load for the cache
  is approximately duplicated by each instance.

    * If T is the current total size of the cache (the sum of the
      sizes of all distinct cached responses), then the memory load at
      each of N replicas in the cluster is approximately T, and the
      total load in the cluster is approximately T*N.

* For N replicas in the Varnish cluster, cacheable responses are
  fetched N times from the Services that send them; and they are
  re-fetched N times when the TTL expires.

* Even after the response to a request has been cached, the next
  request for the same object may be a cache miss, if the LoadBalancer
  happens to forward the request to a Varnish instance that doesn't
  have it yet in its cache.

* If a cached response changes after its TTL elapses, then the
  instances in the cluster may return different cached responses to
  the same request, if some of them have the version before the
  change, and others after the change.

The purpose of self-sharding is to:

* distribute memory load for the cache among the instances in the
  cluster.

    * If T is the total cache size as described above, then the total
      memory load in the cluster is approximately T, and the load at
      each replica is approximately T/N.

* reduce the request load on Services, so that there is only one fetch
  for a cacheable response, from only one Varnish instance, until the
  TTL expires.

* ensure that if a cached response has been fetched from a Service
  just once, then any further request for the same object will be a
  cache hit until the TTL expires, regardless of which Varnish
  instance receives the request from the LoadBalancer. The cache hit
  will always be the same response -- the object most recently fetched
  from a Service.

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

This is done by applying Varnish's
[shard director](https://varnish-cache.org/docs/6.3/reference/vmod_directors.generated.html#new-xshard-directors-shard)
to the Varnish instances in the cluster. If an instance finds that the
director shards the request to itself, then it handles the request
itself as the primary cache for the request. See the documentation at
the link for more details about the shard director.

If a request is evaluated so that the response won't be cacheable in
any case (such as POST requests in default configurations), then the
request is forwarded to the Service directly, since there is no point
in forwarding it to another cache.

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
  load for the cache in the cluster is reduced, while mutliple copies
  of frequently requested objects are still kept for low response
  latencies.

* A cacheable response is fetched from a Service only once, from the
  instance that handles the request itself without forwarding. When
  the TTL expires, it is re-fetched only once from the same instance.

* If a cacheable response is in at least one the cluster's caches,
  then subsequent requests for the same object will be cache hits
  while the TTL is still valid, regardless of which Varnish instance
  received the request from the LoadBalancer. Downstream responses are
  consistent, since there is one primary cache for each response.

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

## Configuration

Self-sharding is configured by specifying a ``self-sharding`` object
in a ``VarnishConfig`` Custom Resource, and naming the Service names
for Varnish Services running as Ingress implementations in the same
namespace as the Custom Resource. For example:

```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: VarnishConfig
metadata:
  name: self-sharding-cfg
spec:
  services:
    - my-ingress
  self-sharding:
    max-secondary-ttl: 3m
    probe:
      timeout: 6s
      interval: 6s
      initial: 2
      window: 4
      threshold: 3
```

VCL to implement self-sharding is generated by the controller for
Varnish-as-Ingress clusters running as a Service named in
``services``.

Self-sharding is implemented when the ``self-sharding`` object is
present. Properties of the ``self-sharding`` object are all optional,
and default values hold if they are left out of the configuration. To
configure self-sharding with all default values, just specify an empty
object:

```
spec:
  # Self-sharding with all properties set to defaults.
  self-sharding: {}
```

The ``max-secondary-ttl`` parameter defaults to ``5m`` (5 minutes),
and sets the upper bound for "secondary" caching as discussed
above. If an instance forwards a request to another instance and the
response is cacheable, then the forwarding instance may also cache the
response (as does the "primary" instance). But the secondary instance
will not retain the response for longer than the value of
``max-secondary-ttl`` (it is likely to hit the primary cache after
that TTL expires). Keeping this value low relative to the "primary"
TTLs serves to reduce the total memory load for caching in the
cluster.

The ``probe`` object specfies properties of the
[health probes](https://varnish-cache.org/docs/6.3/reference/vcl.html#probes),
that Varnish instances in the cluster send to one another (since they
are backends for one another when self-sharding is implemented).

See the [technical reference](/docs/ref-varnish-cfg.md#spec-self-sharding)
for ``VarnishConfig`` for details of the configuration, and the
[``examples/``](/examples/self-sharding) folder for a working example.
