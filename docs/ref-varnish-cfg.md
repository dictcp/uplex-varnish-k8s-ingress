# VarnishConfig Custom Resource reference

This is the authoritative reference for the ``VarnishConfig``
[Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
which is defined in this project to specify special configurations and
features for Varnish clusters running as Ingress, in addition to the
routing rules given in an Ingress specification.

If you only intend to use Varnish-as-Ingress Services to implement the
routing rules for an Ingress, then no ``VarnishConfig`` resource needs to
be defined. Use the ``VarnishConfig`` resource to apply additional
configurations such as [self-sharding](/docs/self-sharding.md).

Constraints on individual properties in ``VarnishConfig`` are checked
against validation rules when the manifest is applied, so you may get
immediate feedback about invalid values from a ``create`` or ``apply``
command for ``kubectl``. Other constraints, such as legal relations
between values or valid VCL syntax, cannot currently be checked until
the controller attempts to load the definition, and hence will not be
reported at apply time. Check the log of the controller and Events
created by the controller for error conditions -- these may include
error messages from the VCL compiler.

Examples for the use of ``VarnishConfig`` resources can be found in
the [``examples/``](/examples) folder.

## Custom Resouce definition

The Custom Resource is created with the ``CustomResourceDefinition`` defined
in [``varnishcfg-crd.yaml``](/deploy/varnishcfg-crd.yaml) in the
[``deploy/``](/deploy) folder:

```
$ kubectl apply -f deploy/varnishcfg-crd.yaml
```

## API Group, version and resource names

The API group in use for this project is
``ingress.varnish-cache.org``, currently at version ``v1alpha1``. So a
manifest specifying a ``VarnishConfig`` resource MUST begin with:
```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: VarnishConfig
```

You can choose any ``name`` and ``namespace`` in the ``metadata``
section.  ``VarnishConfig`` has Namespaced scope, so its name must be
unique in a namespace, and its content is applied to Varnish Services
in the same namespace.

Existing ``VarnishConfig`` resources can be referred to in ``kubectl``
commands as ``varnishconfig``, ``varnishconfigs`` or with the short
name ``vcfg``:

```
$ kubectl get varnishconfigs -n my-namespace
$ kubectl describe vcfg my-vcfg
```

## ``spec``

The ``spec`` section of a ``VarnishConfig`` is required.

### ``spec.services``

The ``spec.services`` array is required, and MUST have at least one
element:

```
spec:
  # The services array is required and must have at least one element.
  # Lists the Service names of Varnish services in the same namespace
  # to which this config is to be applied.
  services:
    - my-ingress
```

The strings in the ``services`` array MUST match the Service names of
Varnish Services to implement Ingress in the same namespace as the
``VarnishConfig`` Resource. The configuration in the Resource is
applied to those resources -- this makes it possible to have more than
one Varnish-as-Ingress Service in a namespace with different
configurations.

### ``spec.self-sharding``

The ``self-sharding`` object is optional. If it is present in the
manifest, then the [self-sharding](/docs/self-sharding.md) feature is
implemented for Services listed in the ``services`` array.

All of the properties of a ``self-sharding`` object are optional, and
default values hold for any properties that are not specified. To
specify self-sharding with all default values, just use an empty object:

```
spec:
  # Implement self-sharding with defaults for all properties:
  self-sharding: {}
```

Properties that may be specifed for ``self-sharding`` are:

* ``max-secondary-ttl``: string
* ``probe``: object

If specified, ``max-secondary-ttl`` MUST have the form of the VCL
[DURATION type](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)
(examples are ``90s`` for ninety seconds, or ``2m`` for two
minutes). This value is the TTL for "secondary" caching -- the upper
bound for a cached response forwarded from the "primary" Varnish instance
for a cacheable response (see the
[self-sharding document](/docs/self-sharding.md) for details).
``max-secondary-ttl`` defaults to ``5m`` (5 minutes).

The ``probe`` object specifies the health probes that Varnish instances
in a cluster use for one another (since they are defined as backends
for one another). Its properties are:

* ``timeout``: string
* ``interval``: string
* ``initial``: integer
* ``window``: integer
* ``threshold``: integer

These properties configure the corresponding values for
[health probes](https://varnish-cache.org/docs/6.1/reference/vcl.html#probes),
and they default to the default values for Varnish probes. If the
``probe`` object is left out altogether, then defaults hold for all of
its properties.

``timeout`` and ``interval`` MUST have the form of VCL DURATIONs, and
each of ``initial``, ``window`` and ``threshold`` MUST be >= 0.
``window`` and ``threshold`` MUST also be <= 64, and ``threshold``
MAY NOT be larger than ``window``.

Validation for ``VarnishConfig`` will report errors in the individual
fields at apply time, for example if the VCL DURATION properties do
not have the proper form. The ``threshold`` <= ``window`` constraint
is checked at VCL load time; if violated, it is reported in the
controller log and in Events generated by the controller for the
``VarnishConfig`` resource (with the error message from the VCL
compiler).

Example:
```
spec:
  self-sharding:
    # Any of these properties may be left out, in which case default
    # values hold.
    max-secondary-ttl: 2m
    probe:
      timeout: 6s
      interval: 6s
      initial: 2
      window: 4
      threshold: 3
```
