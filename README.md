# Varnish Ingress Controller

This is an implementation of a [Kubernetes Ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress/)
based on [Varnish](http://www.varnish-cache.org).

The present documentation presupposes familiarity with both Kubernetes and
Varnish. For more information, see:

* Kubernetes: https://kubernetes.io/
* Varnish: http://www.varnish-cache.org

The Ingress controller currently supports Kubernetes version 1.9, and
has also been tested succesfully with 1.10. The Varnish container runs
version 6.1.1.

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

# Documentation

See the [``docs/``](/docs) folder for technical references and more
detailed discussions of various topics.

# Repositories

* Primary repo: https://code.uplex.de/uplex-varnish/k8s-ingress

* Mirror: https://gitlab.com/uplex/varnish/k8s-ingress
