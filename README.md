# Varnish Ingress Controller

This is an implementation of a [Kubernetes Ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress/)
based on [Varnish](http://www.varnish-cache.org).

The present documentation presupposes familiarity with both Kubernetes and
Varnish. For more information, see:

* Kubernetes: https://kubernetes.io/
* Varnish: http://www.varnish-cache.org

## WORK IN PROGRESS

The Ingress controller implementation is presently under development
as a minimum viable product (MVP) and is undergoing initial testing. It is
currently subject to a number of limitations, expected to be removed over
time, including:

* No support for TLS connections
* The controller only attends to definitions (Ingresses, Services and
  Endpoints) in the namespace of the Pod in which it is deployed.
* Only one Ingress definition is valid at a time. If more than one definition
  is added to the namespace, then the most recent definition becomes valid.
* A variety of elements in the implementation are hard-wired, as
  detailed in the documentation, These are expected to become configurable
  in further development.

# Installation

The Ingress controller currently supports Kubernetes version 1.9, and
has also been tested succesfully with 1.10.

Varnish for the purposes of Ingress and the controller that manages it
are implemented in separate containers -- one controller can be used
to manage a group of Varnish instances. The Dockerfiles and other
files needed to build the two images are in the
[``container/``](/container) folder, together with a Makefile that
encapsulates the commands for the build.

The resulting images must then be pushed to a registry available to
the Kubernetes cluster.

The Ingress can then be deployed by any of the means that are
customary for Kubernetes. The [``deploy/``](/deploy) folder contains
manifests (YAMLs) for some of the ways to deploy an Ingress.

The [``examples/``](/examples) folder contains YAMLs for Services and
Ingresses to test and demonstrate the Varnish implementation and its
features. You might want to begin with the
["cafe" example](/examples/hello) inspired by other projects (a kind
of "hello world" for Ingress).

This implementation requires that the Ingress definition includes an
``ingress.class`` Annotation identifying ``varnish``:
```
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: "varnish"
[...]
```
The controller ignores all Ingress definitions that do not include the
annotation. So you can work with other Ingress controllers that are
based on other technologies in the same Kubernetes cluster.

# Development

The source code for the controller, which listens to the k8s cluster
API and issues commands to Varnish instances to realize Ingress
definitions, is in the [``cmd/``](/cmd) folder. The folder also
containes a Makefile defining targets that encapsulate the build
process for the controller executable.

# Varnish as a Kubernetes Ingress

Since this project is currently in its early stages, the implementation of
Ingress definitions for a Varnish instance is subject to change as development
continues. Presently, an Ingress is realized by loading a
[VCL](https://varnish-cache.org/docs/trunk/reference/vcl.html) configuration
that:

* defines
  [directors](https://varnish-cache.org/docs/trunk/users-guide/vcl-backends.html#directors)
  that correspond to Services mentioned in the Ingress definition
  * This is currently hard-wired as the round-robin director.
* defines
  [backends](https://varnish-cache.org/docs/trunk/users-guide/vcl-backends.html)
  corresponding to the Endpoints of the Services. These are assigned to the
  director defined for the Service.
  * Endpoint definitions for a Service are obtained from an API query
    when the VCL configuration is generated, so that the assignments
    of Endpoints is current at VCL generation time.
  * The Varnish backend definitions currently only include the Endpoints'
    IP addresses and ports. There is presently no means to define other
    features of a backend, such as health checks and timeouts.
* generates VCL code to implement the routing of requests to backends
  based on the Host header and/or URL path according to the IngressRules
  given in the Ingress definition.
  * Varnish may cache responses according to its default rules for
    caching, and of course cache hits are delivered without routing the
    requests further.
  * In case of a non-cache-hit (miss or pass), the Host header and/or
    URL path is evaluated according to the IngressRules, and matching
    requests are assigned to the director corresponding to the matched
    Service.
      * An IngressSpec is rejected if it does not specify any Host header.
      * TLS configuration in the IngressSpec is currently ignored.
  * The director in turn chooses a backend corresponding to an Endpoint
    according to its load balancing algorithm (currently only round-robin).
  * If the request does not match any Service according to the
    IngressRules, then:
      * If a default Backend (a Service) was defined in the IngressSpec,
        then the request is assigned to the corresponding director.
      * Otherwise, a synthetic 404 Not Found response is generated by
        Varnish.
  * If there is no valid Ingress definition (none has been defined
    since the Varnish instance started, or the only valid definition
    was deleted), then Varnish generates a synthetic 503 Service Not
    Available response for every request.
