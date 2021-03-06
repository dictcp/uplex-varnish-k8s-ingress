# Structuring Varnish Services, Ingresses, controllers and namespaces

This document is the authoritative reference for the configuration
elements and rules governing these relationships:

* which Varnish Service implements the routing rules of an Ingress
  definition

* how Varnish Services, Ingress and backend Services from different
  namespaces can be related

* how various Ingress definitions can be merged into a comprehensive
  set of routing rules implemented by a single Varnish Service

* how to operate more than one controller in a cluster, if needed

These relations are driven by the contents of Ingress definitions,
both their rules and these two annotations:

* ``kubernetes.io/ingress.class``: specifies whether the controller
  considers the Ingress for implementation by Varnish

* ``ingress.varnish-cache.org/varnish-svc``: optionally specifies
  the Varnish Service to implement the rules in an Ingress definition

For example:

```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: coffee-ingress
  namespace: cafe
  annotations:
    kubernetes.io/ingress.class: "varnish"
    ingress.varnish-cache.org/varnish-svc: "varnish-coffee"
spec:
[...]
```

See the [``examples/`` folder](/examples/architectures/) for sample
configurations that apply the following rules.

* The controller only considers Ingress definitions with the
  ``kubernetes.io/ingress.class`` annotation set to specify Varnish as
  the implementation, by default with the value ``"varnish"`` (or the
  value of the [controller option
  ``-class``](/docs/ref-cli-options.md)).  Ingresses that do not have
  the annotation, or in which the annotation is set to another value,
  are ignored.

* Services that run Varnish and implement Ingress, using the
  Varnish container defined for this project, are identified
  with the label ``app:varnish-ingress``.

* An Ingress definition may have the annotation
  ``ingress.varnish-cache.org/varnish-svc`` to specify the Varnish
  Service that implements its rules, by name and optionally by
  namespace (as ``"namespace/name"`` or just ``"name"``). If the
  annotation value does not specify a namespace, then the same
  namespace as the Ingress is assumed.

* If an Ingress definition does not have the ``varnish-svc``
  annotation, then:

    * if there is only one Varnish Service (with the
      ``app:varnish-ingress`` label) in the entire cluster, then that
      Service implements the Ingress rules.

    * otherwise if there is only one Varnish Service in the same
      namespace as the Ingress, then that Service implements its
      rules.

    * otherwise the Ingress definition is rejected as an error.

* A Varnish Service may have Services from different namespaces as its
  backends, if it implements Ingress definitions from those namespaces
  that specify the Services as Ingress backends.  (An Ingress can only
  specify backends in its own namespace.)

* These rules make it possible to merge various Ingress definitions
  into a set of combined Ingress rules implemented by a Varnish
  Service. This is permitted if:

    * No host name appears in more than one of the Ingress definitions
      to be merged.

    * There is no more than one default Ingress backend in all of the
      Ingress definitions to be merged.

  An Ingress is rejected as an error if it would violate either of
  these restrictions for a merge.

Non-overlapping hosts in different Ingresses are not permitted because
of the Kubernetes standard specification for host and path rules. For
each host, the first path rule that matches the URL determines how a
request is routed. But if the same host appears in more than one
Ingress, then there is no defined ordering for the path rules.

## Multiple controllers

The controller is designed so that it can run in only one Pod and
manage all of the Varnish Services for Ingress in the entire cluster
(deployment in namespace ``kube-system`` is a natural choice). But it
is possible to run more than one instance to manage separate Varnish
Services, for example to partition the controller load, or to
logically separate the responsibilities of controllers.

To do so:

* Start the different controller instances with different values of
  the [command-line option ``-class``](/docs/ref-cli-options.md), to
  designate distinct values of the Ingress annotation
  ``kubernetes.io/ingress.class``. Then the different controller
  instances will only implement the Ingress definitions that have
  their "own" value for the annotation.

* Ingress definitions with distinct values of the ``ingress.class``
  annotation should designate distinct Varnish Services (with one of
  the means described above). In other words, the Ingresses and
  Varnish Services managed by one controller should not be managed by
  any other controller.

Multiple controllers in a cluster SHOULD NOT be started with the same
value of the ``-class`` option. Varnish Services SHOULD NOT be
designated by Ingress definitions with different values of the
``ingress.class`` annotation. If more than one controller attempts to
manage the same Ingresses or Varnish Services, the results are
undefined, and the desired state of the cluster might not be achieved.

See the [``examples/`` folder](/examples/architectures/multi-controller/)
for a working example of two Varnish controllers in a cluster.
