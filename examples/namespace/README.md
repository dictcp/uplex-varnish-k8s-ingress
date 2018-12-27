# Restricting the Ingress controller to a single namespace

This folder contains an example of the use of the
[``-namespace`` option](/docs/ref-cli-options.md) of the Ingress
controller to limit its actions to Services, Ingresses, Secrets and so
on to a single namespace. The controller can then be deployed in that
namespace. You may need to do so, for example, due to limits on
authorization in the cluster.

The sample manifests use ``varnish-ingress`` as the example namespace,
and re-uses the ["cafe" example](/examples/hello), with Services and
an Ingress defined for the namespace.

In the following we follow the steps in the documentation for a
[cluster-wide deployment of the Ingress controller](/deploy) and the
[deployment of the example](/examples/hello). Only the details
concerning the single-namespace deployment are covered here; see the
other doc links for further information.

## Deployment

### Namespace and ServiceAccount

Define the Namespace ``varnish-ingress``, and a ServiceAccount named
``varnish-ingress`` in that namespace:
```
$ kubectl apply -f ns-and-sa.yaml
```

All further operations will be restricted to the Namespace.

### RBAC

The RBAC manifest grants the same authorizations as for the
[cluster-wide deployment](/deploy), but they are bound to the
ServiceAccount just defined. This allows the Ingress controller
to operate in the sample Namespace (only).
```
$ kubectl apply -f rbac.yaml
```

### Admin Secret, Varnish deployment and Nodeport

These are created by this sequence of commands:
```
$ kubectl apply -f adm-secret.yaml
$ kubectl apply -f varnish.yaml
$ kubectl apply -f nodeport.yaml
```
The manifests only differ from their counterparts from the
[cluster-wide deployment instructions](/deploy) in the namespace.

### Controller

To run the controller container:
```
$ kubectl apply -f controller.yaml
```
This manifest differs from a cluster-wide deployment in that:

* The ``serviceAccountName`` is assigned the ServiceAccount defined
  above.
* The container is invoked with the ``-namespace`` option to limit its
  work to the given namespace:

```
    spec:
      serviceAccountName: varnish-ingress
      containers:
      - args:
        # Controller only acts on Ingresses, Services etc. in the
        # given namespace.
        - -namespace=varnish-ingress
```

The controller now only watches Services, Ingresses, Secrets and so
forth in the ``varnish-ingress`` namespace.

## Example

The "cafe" example can be deployed with:
```
$ kubectl apply -f cafe.yaml
$ kubectl apply -f cafe-ingress.yaml
```
Their manifests differ from those of the ["hello" example](/examples/hello)
in their restriction to the sample namespace.
