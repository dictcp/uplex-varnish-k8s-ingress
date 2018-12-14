# Self-sharding Varnish cluster

The manifest in this folder is an example configuration for an Ingress
with annotations for sharding the cache in a Varnish cluster. See
[the documentation](/docs/self-sharding.md) for details.

The Ingress may be deployed with the Services from
[the "cafe" example](/examples/hello).

```
$ kubectl apply -f cafe-ingress-selfshard.yaml
```

Its rules specification is the same as those in the Ingress from the cafe
example, but it contains annotations to configure self-sharding:

```
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
```

Only the first annotation (``self-sharding``) is required to activate
the feature; the others are optional, and can edited for your
requirements, or removed to be left to default values.
