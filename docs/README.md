# Documentation

The docs in this folder cover these topics:

* Technical references: authoritative documentation for these subjects:

  * [``VarnishConfig`` Custom Resource](ref-varnish-cfg.md)
  * [``BackendConfig`` Custom Resource](ref-backend-cfg.md)
  * [controller command-line options](ref-cli-options.md)
  * [customizing the Pod template](varnish-pod-template.md) for Varnish
  * [metrics](ref-metrics.md) published by the controller

* [Logging, Events and the Varnish Service monitor](monitor.md)

* [Varnish as a Kubernetes Ingress](varnish-as-ingress.md)

* [Custom VCL](/docs/custom-vcl.md): restrictions, conventions, and
  links to further information about VCL

* [Self-sharding Varnish cluster](self-sharding.md): High-level
  discussion of the design

* [Developer documentation](dev.md) -- generating code; and building,
  testing and maintaining the controller executable.
