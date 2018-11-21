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
  hard-wired, as detailed in the following, These are expected to
  become configurable in further development.

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
customary for Kubernetes. The [``deploy/``](/deploy) folder contains
YAML configurations for one of the ways to deploy an Ingress.

The [``examples/``](/examples) folder contains YAML configurations for
sample Services and an Ingress to test and demonstrate the Ingress
implementation (based on the "cafe" example from other projects).

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

The executable ``k8s-ingress``, which acts as the Ingress controller,
is currently built with Go 1.10.

Targets in the Makefile support development in your local environment, and
facilitate testing with ``minikube``:

* ``k8s-ingress``: build the controller executable. This target also
  runs ``go get`` for package dependencies, ``go generate`` (see
  below) and ``go fmt``.

* ``check``, ``test``: build the ``k8s-ingress`` executable if
  necessary, and run ``go vet``, ``golint`` and ``go test``.

* ``clean``: run ``go clean``, and clean up other generated artifacts

If you are testing with ``minikube``, set the environment variable
``MINIKUBE=1`` before running ``make container``, so that the
container will be available to the local k8s cluster:

```
$ MINIKUBE=1 make container
```

The build currently depends on the tool
[``gogitversion``](https://github.com/slimhazard/gogitversion) for the
generate step, to generate a version string using ``git describe``,
which needs to be installed by hand. This sequence should suffice:

```
$ go get -d github.com/slimhazard/gogitversion
$ cd $GOPATH/src/github.com/slimhazard/gogitversion
$ make install
```
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
    was deleted), then Varnish generates a synthetic 404 Not Found
    response for every request.
