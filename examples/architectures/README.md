# Architectures for Varnish Services, Ingresses, controllers and namespaces

The examples in the subfolders illustrate some of the possible
relations between Ingress controllers, Varnish Services implementing
Ingress, Ingress definitions defining routing rules, and the
namespaces in which they are deployed:

* A [cluster-wide Varnish
  Service](/examples/architectures/clusterwide/) that implements
  Ingress rules in all namespaces.

* A [setup](/examples/architectures/cluster-and-ns-wide/) with a
  cluster-wide Service, and another Varnish Service that implements
  Ingress rules in its own namespace.

* [Multiple Varnish
  Services](/examples/architectures/multi-varnish-ns/) in the same
  namespace, each of which implement separate Ingress rules.

* [Multiple Ingress
  controllers](/examples/architectures/multi-controller/) for Varnish,
  managing separate sets of Varnish Services and Ingresses.

These configurations apply the [rules](/docs/ref-svcs-ingresses-ns.md)
concerning the relationships between Varnish Services, Ingresses and
namespaces.
