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
* A variety of elements in the Varnish implementation of Ingress are
  hard-wired, as detailed of the following, and are expected to become
  configurable in further development.

# Installation

The container image implementing the Ingress, including the Ingress
controller deployed in the same container, is created via a
multi-stage Docker build using the Dockerfile in the root of the
source repository. The build can be initiated with the ``container``
target for ``make``:

```
$ make container
```

If you wish to add custom options for the Docker build, assign these
to the environment variable ``DOCKER_BUILD_OPTIONS``:

```
$ DOCKER_BUILD_OPTIONS='--no-cache --pull' make container
```

The resulting image must then be pushed to a registry available to the
Kubernetes cluster.

The Ingress can then be deployed by any of the means that are
customary for Kubernetes. The [``deploy/``](deploy/) folder contains YAML
configurations for one of the ways to deploy an Ingress.

The [``example/``](example/) folder contains YAML configurations for
sample Services and an Ingress to test and demonstrate the Ingress
implementation (based on the "cafe" example from other projects).
