# Self-sharding Varnish cluster

The manifest in this folder is an example specification for a
``VarnishConfig`` Custom Resource that defines sharding the cache in a
Varnish cluster. See [the documentation](/docs/self-sharding.md) for a
high-level discussion of the concept.

```
$ kubectl apply -f self-sharding-cfg.yaml
```

The YAML specifies the API group and version for the Custom Resource,
and ``VarnishConfig`` as the ``kind``:
```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: VarnishConfig
```

The ``spec`` section of the ``VarnishConfig`` manifest MUST include the
``services`` array, which MUST have at least one element. Strings in
this array name Services in the same namespace in which the Custom
Resource is defined, identifying the Varnish-as-Ingress Services to
which the configuration is to be applied:

```
  # Apply the configuration to the Service 'varnish-ingress' in the
  # same namespace.
  services:
    - varnish-ingress
```

Self-sharding is applied if the ``self-sharding`` object is present in
the ``VarnishConfig`` resource. All of its config elements are optional,
and default values hold if they are left out. To just specify self-sharding
with all defaults, include ``self-sharding`` in the manifest as an
empty object:

```
  # Implement self-sharding in the Varnish Services with all default
  # options.
  self-sharding: {}
```

The sample YAML sets values for all of the possible options:

```
  self-sharding:
    max-secondary-ttl: 2m
    probe:
      timeout: 6s
      interval: 6s
      initial: 2
      window: 4
      threshold: 3
```

Only the first annotation (``self-sharding``) is required to activate
the feature; the others are optional, and can edited for your
requirements, or removed to be left to default values.
