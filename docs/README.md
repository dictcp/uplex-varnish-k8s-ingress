# Documentation

The docs in this folder cover these topics:

* Technical references: authoritative documentation for these subjects:

  * [``VarnishConfig`` Custom Resource](ref-varnish-cfg.md)
  * [``BackendConfig`` Custom Resource](ref-backend-cfg.md)
  * [controller command-line options](ref-cli-options.md)
  * [customizing the Pod template](varnish-pod-template.md) for Varnish
  * [metrics](ref-metrics.md) published by the controller
  * [configuration elements and rules](ref-svcs-ingresses-ns.md) for:
      * specifying the Varnish Service that implements the routing
        rules of an Ingress definition
      * relating Varnish Services, Ingresses and backend Services from
        different namespaces
      * merging various Ingress definitions into a comprehensive set
        or routing rules implemented by a Varnish Service
      * running more than one controller in a cluster, if necessary
        (in most cases, one controller Pod in a cluster will suffice)

* [Logging, Events and the Varnish Service monitor](monitor.md)

* [Varnish as a Kubernetes Ingress](varnish-as-ingress.md)

* [Custom VCL](/docs/custom-vcl.md): restrictions, conventions, and
  links to further information about VCL

* [Self-sharding Varnish cluster](self-sharding.md): High-level
  discussion of the design

* [Developer documentation](dev.md) -- generating code; and building,
  testing and maintaining the controller executable.
