# BackendConfig Custom Resource reference

This is the authoritative reference for the ``BackendConfig``
[Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
which is defined in this project to specify the Varnish configuration
for Services that are named as a ``backend`` in an Ingress definition;
that is, the Services to which requests are routed according to an
Ingress based on the Host header and/or URL path. These are implemented
as
[backends](https://varnish-cache.org/docs/6.1/users-guide/vcl-backends.html)
in Varnish, and BackendConfig allows you to set features for them
such as timeouts, health probes, and the load balancing algorithm.

If no BackendConfig is defined for such a Service, then the backend
configuration is set with default values.

Constraints on individual properties in ``BackendConfig`` are checked
against validation rules when the manifest is applied, so you may get
immediate feedback about invalid values from a ``create`` or ``apply``
command for ``kubectl``. Other constraints, such as legal relations
between values, cannot currently be checked until the controller
attempts to load the definition, and hence will not be reported at
apply time. Check the log of the controller and Events created by the
controller for error conditions.

Working examples of ``BackendConfig`` resources can be found in the
[``examples/``](/examples/backend-config) folder.

## Custom Resouce definition

The Custom Resource is created with the ``CustomResourceDefinition`` defined
in [``backendcfg-crd.yaml``](/deploy/backendcfg-crd.yaml) in the
[``deploy/``](/deploy) folder:

```
$ kubectl apply -f deploy/backendcfg-crd.yaml
```

## API Group, version and resource names

The API group in use for this project is
``ingress.varnish-cache.org``, currently at version ``v1alpha1``. So a
manifest specifying a ``BackendConfig`` resource MUST begin with:
```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: BackendConfig
```

You can choose any ``name`` and ``namespace`` in the ``metadata``
section.  ``BackendConfig`` has Namespaced scope, so its name must be
unique in a namespace, and its content is applied to Varnish Services
in the same namespace.

Existing ``BackendConfig`` resources can be referred to in ``kubectl``
commands as ``backendconfig``, ``backendconfigs`` or with the short
name ``becfg``:

```
$ kubectl get backendconfigs -n my-namespace
$ kubectl describe becfg my-becfg
```

## ``spec``

The ``spec`` section of a ``BackendConfig`` is required.

### ``spec.services``

The ``spec.services`` array is required, and MUST have at least one
element:

```
spec:
  # The services array is required and must have at least one element.
  # Lists the names of Services in the same namespace. If the Service
  # is specified as a backend in an Ingress to be implemented by Varnish,
  # then the BackendConfig is applied to it.
  services:
    - my-svc
```

The strings in the ``services`` array MUST match the names of Services
in the same namespace as the ``BackendConfig`` Resource. The
configuration in the Resource is applied to those Services -- this
makes it possible to apply the same BackendCondig to more than one
Service that forms a ``backend`` in an Ingress.

### ``spec`` top-level properties

The ``spec`` object may have any of these properties, all optional,
which correspond to attributes of a
[Varnish backend configuration](https://varnish-cache.org/docs/6.1/reference/vcl.html#backend-definition):

* ``host-header``: non-empty string

* ``connect-timeout``: MUST have the form of a
  [VCL DURATION](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)

* ``first-byte-timeout``: VCL DURATION

* ``between-bytes-timeout``: VCL DURATION

* ``proxy-header``: integer 1 or 2 (for the PROXY protocol version)

* ``max-connections``: positive integer

If any of these properties are left out, then Varnish defaults hold
for the backend.

For example:

```
spec:
  services:
    - tea-svc

  # Set timeouts and max connections for the Service tea-svc.
  connect-timeout: 1s
  first-byte-timeout: 2s
  between-bytes-timeout: 1s
  max-connections: 200
```

### ``spec.probe``

The ``probe`` object is optional, and if present it specifies a
[health probe](https://varnish-cache.org/docs/6.1/reference/vcl.html#probes)
for the backend. Its properties correspond to attributes of a Varnish
probe:

* ``url``: URL path for the probe (MUST begin with "/")

* ``request`` (array of non-empty strings): the full HTTP request, in
  which each element of the array forms a line in the request; details
  below.

* ``expected_response``: 3-digit HTTP response code

* ``timeout``: MUST have the form of a
  [VCL DURATION](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)

* ``interval``: VCL DURATION

* ``initial``: non-negative integer

* ``window``: integer >= 0 and <= 64

* ``threshold``: integer >= 0 and <= 64

Only one of ``url`` or ``request`` may be set for a health probe; if a
BackendConfig has both, then ``url`` is used, and ``request`` is
ignored. (In future versions, such a configuration may be rejected as
invalid, so it's advisable to use only one or the other in the current
version.)

If ``request`` is specified, then the strings in the array are sent as
lines in the health probe request, separated by ``\r\n``. For example:

```
# Health probe configuration in a BackendConfig:
spec:
  probe:
    request:
    - GET /coffee/healthz HTTP/1.1
    - "Host: cafe.example.com"
    - "Connection: close"

# The health probe request is sent as:
GET /coffee/healthz HTTP/1.1\r\n
Host: cafe.example.com\r\n
Connection: close\r\n
\r\n
```

Note that a line with a request header must be explicitly quoted.
Otherwise it is interpreted as a YAML object and rejected, since the
array may only consist of strings.

If any properties of ``spec.probe`` are left out, then Varnish
defaults hold for the corresponding attribute of the probe. To just
specify a probe with all default values (using ``.url`` with its
default value ``/``), then use an empty YAML object:

```
# Health probe with all default values
spec:
  probe: {}
```

In addition to the constraints described above, ``threshold`` MAY NOT
be larger than ``window``. Validation for ``BackendConfig`` will
report errors in the individual fields at apply time, for example if
the VCL DURATION properties do not have the proper form. The
``threshold`` <= ``window`` constraint is currently checked at VCL
load time; if violated, it is reported in the controller log and in
Events generated by the controller for the ``BackendConfig`` resource
(with the error message from the VCL compiler).

Example:
```
spec:
  # Health probe config
  # see: https://varnish-cache.org/docs/6.1/reference/vcl.html#probes
  probe:
    url: /tea/healthz
    expected-response: 204
    timeout: 5s
    interval: 5s
    initial: 1
    window: 3
    threshold: 2
```

### ``spec.director``

The ``director`` object is optional, and if present it specifies
properties of the
[Varnish director](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html)
that corresponds to the backend Service. Varnish directors implement
load-balancing for a group of backends; for the Ingress
implementation, Varnish routes requests to one of the Endpoints of the
Service indicated by the request routing rules, and the director
chooses the Endpoint.

All of the properties of ``spec.director`` are optional:

* ``type``: one of ``round-robin``, ``random`` or ``shard``, default
  ``round-robin``

* ``warmup`` (integer 0 to 100): the
  [``warmup`` parameter](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html#func-shard-set-warmup)
  of the ``shard`` director, expressed as a probability in percent.
  Ignored for the other directors.

* ``rampup`` ([VCL DURATION](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)):
  the
  [``rampup`` parameter](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html#void-xshard-set-rampup-duration-duration-0)
  of the ``shard`` director. Ignored for the other directors.

With ``type`` you can choose the
[round-robin](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html#obj-round-robin),
[random](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html#obj-random)
or
[shard](https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html#obj-shard)
director, default round-robin. The shard director shards requests
to Endpoints by URL path.

The ``warmup`` and ``rampup`` parameters are only relevant for the
shard director, and serve to mitigate the impact of adding or removing
Endpoints from the Service. This may be useful, for example, for
applications that have their own data caches, and may become
distressed if they receive too many requests for uncached data too
rapidly.

``warmup`` is the probability in percent that, instead of choosing the
"first" Endpoint to which a request is ordinarily sharded, the
director chooses the next Endpoint that would be chosen if the first
Endpoint is removed. Then if the first Endpoint is removed, the "next"
Endpoint has already received some of the requests that the director
now sends to it. This allows an application resource such as a cache
to be "pre-warmed" for some of the new requests it receives.

``rampup`` is a time interval that begins when a new Endpoint is
added. During that time, the director chooses the "next" Endpoint
rather than the new Endpoint for a request, with a probability that is
100% when the Endpoint is added, and decreases linearly to 0% at the
end of the rampup interval. This mitigates the "thundering herd" of
requests for a newly added Endpoint.

For example:

```
spec:
  # Use the shard director, with a "warmup" probability of 50% (or
  # 0.5), and a five-minute rampup interval.
  director:
    type: shard
    warmup: 50
    rampup: 5m
```

For the Ingress implementation, a director is always configured,
round-robin by default. So if the default is sufficient for your
requirements, you can just leave out ``spec.director`` from the
BackendConfig.

See the [docs](/docs/custom-vcl.md) for conventions and restrictions
that apply to custom VCL, and for links to more information about VCL.
