# Sample BackendConfig resources

The sample manifest in this folder configures BackendConfig resources,
to specify properties of Services and their Endpoints when they are
implemented as Varnish backends. These are Services named in the
``backend`` field of an Ingress, to which requests are routed
according to the Host header and/or URL path. See the
[docs](/docs/ref-backend-cfg.md) for the tecnhinal reference.

The example applies to the Services defined in the
["cafe" example](/examples/hello) -- the ``coffee-svc`` and ``tea-svc``.

Apply the configurations with:

```
$ kubectl apply -f backend-cfg.yaml
```

First consider the BackendConfig for ``tea-svc``:

```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: BackendConfig
metadata:
  name: tea-svc-cfg
spec:
  services:
    - tea-svc
[...]
```

The ``services`` array is required, must have at least one element,
and must name Services in the same namespace in which the
BackendConfig is defined. When a Service in the array is named as the
``backend`` for an Ingress to be implemented by Varnish, the
BackendConfig is applied to that Service. Listing more than one
Service in the array is way to define one BackendConfig that applies
to several Services.

```
spec:
  [...]
  host-header: cafe.example.com
  connect-timeout: 1s
  first-byte-timeout: 2s
  between-bytes-timeout: 1s
  max-connections: 200
  [...]
```

These top-level properties of ``spec`` correspond to attributes of a
[Varnish backend definition](https://varnish-cache.org/docs/6.3/reference/vcl.html#backend-definition).
The configuration in the example means that:

* The Host header of a backend request is set to ``cafe.example.com``
  if it is missing from the request.

* The connect timeout (for opening new connections to an Endpoint) is
  one second.

* The first byte timeout (until the first byte of a backend response
  header is received) is two seconds.

* The between bytes timeout (while a response is being received) is
  one second.

* No more than 200 connections to an Endpoint may be opened.

The properties of ``spec.probe`` correspond to attributes of a
[Varnish health probe](https://varnish-cache.org/docs/6.3/reference/vcl.html#probes):

```
spec:
  probe:
    url: /tea/healthz
    expected-response: 200
    timeout: 5s
    interval: 5s
    initial: 1
    window: 3
    threshold: 2
```

This defines the health probe requests for Endpoints of tea-svc such
that:

* The URL path of the request is ``/tea/healthz``.

* Health probes are good when the response code is 200.

* Responses time out after five seconds.

* Probes are sent every five seconds.

* Two of three probes must be good for the Endpoint to count as
  healthy.

* At startup, one probe is implicitly assumed to be good.

The last part of the BackendConfig for ``tea-svc`` selects the
[random](https://varnish-cache.org/docs/6.3/reference/vmod_directors.generated.html#new-xrandom-directors-random)
director -- load-balancing requests to Endpoints is random (the
default is round-robin):

```
spec:
  [...]
  director:
    type: random
```

Now consider the BackendConfig for ``coffee-svc``:

```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: BackendConfig
metadata:
  name: coffee-svc-cfg
spec:
  services:
    - coffee-svc
[...]
```

The top-level properties of ``spec`` just set the first-byte and
between-bytes timeouts (other properties are left to the Varnish
defaults):

```
spec:
  [...]
  first-byte-timeout: 3s
  between-bytes-timeout: 2s
```

The ``spec.probe`` configuration sets an explicit request to be used
for health probes (and some of the other attributes):

```
spec:
  [...]
  probe:
    request:
    - GET /coffee/healthz HTTP/1.1
    - "Host: cafe.example.com"
    - "Connection: close"
    timeout: 3s
    interval: 3s
    window: 4
    threshold: 3
```

The strings in the ``request`` array form lines in the probe request,
separated by ``\r\n``, so this probe is sent as:

```
GET /coffee/healthz HTTP/1.1\r\n
Host: cafe.example.com\r\n
Connection: close\r\n
\r\n
```

Note that a line with a request header must be explicitly quoted.
Otherwise it is interpreted as a YAML object and rejected, since the
array may only consist of strings.

The ``spec.director`` configuration specifies the shard director
for load-balancing, and sets the ``warmup`` and ``rampup`` parameters:

```
spec:
  [...]
  director:
    type: shard
    warmup: 50
    rampup: 5m
```

The shard director shards requests to Endpoints by URL. This may be
advantageous, for example, for applications with resources such as
their own data caches, so that cache hits are more likely if requests
with the same URL path are always sent to the same Endpoint.

The ``warmup`` and ``rampup`` parameters serve to mitigate the impact
of adding or removing Endpoints from the Service. ``warmup`` is the
probability in percent that, instead of choosing the "first" Endpoint
to which a request is ordinarily sharded, the director chooses the
next Endpoint that would be chosen if the first Endpoint is
removed. Then if the first Endpoint is removed, the "next" Endpoint
has already received some of the requests that the director now sends
to it. This allows an application resource such as a cache to be
"pre-warmed" for some of the new requests it receives.

``rampup`` is a time interval that begins when a new Endpoint is
added. During that time, the director chooses the "next" Endpoint
rather than the new Endpoint for a request, with a probability that is
100% when the Endpoint is added, and decreases linearly to 0% at the
end of the rampup interval. This mitigates the "thundering herd" of
requests for a newly added Endpoint.

The configuration above means that:

* The "next" Endpoint is chosen by the director with probability 50%.

* Newly added Endpoints have a rampup period of five minutes.
