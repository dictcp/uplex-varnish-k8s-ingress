# Developing the controller

Source code for the controller executable ``k8s-ingress`` is in the
[``cmd/``](/cmd) and [``pkg/``](/pkg) folders, and the
[``Makefile``](/Makefile) in the root of the repository defines
targets for code generation, and for building and maintaining the
controller.

The controller is currently built with Go 1.10. Currently Kubernetes
version 1.9 is supported (it has been also tested successfully with
1.10). This means that the code must be compatible with version 6.0.0
of k8s [client-go](https://github.com/kubernetes/client-go), which in
turn means that it must be compatible with other k8s code required for
client-go 6.0.0.

The controller is deployed in a cluster as the image
``varnish-ingress/controller``, built by the
[Dockerfile](/container/Dockerfile.controller) in the
[``container/``](/container) folder. The controller may also be run
out-of-cluster by launcing it with the ``-kubeconfig`` option to
specify a local Kubernetes config file (for example to test it with
minikube without rebuilding the image):

```
$ ./k8s-ingress -kubeconfig=$HOME/.kube/config
```

Builds are executed with the [``vgo``](https://github.com/golang/vgo)
tool, in anticipation of the
[modules](https://github.com/golang/go/wiki/Modules) feature for
dependency management that is experimental in Go 1.11, and expected to
be finalized in 1.12. ``vgo`` must be installed before you begin with
development:

```
$ go get -u golang.org/x/vgo
```

## Code generation

The project currently uses code generators for two purposes:

* API code generators from
  [``k8s.io/code-generator``](https://github.com/kubernetes/code-generator)
  to generate client APIs for Custom Resources defined by the project,
  such as [``VarnishConfig``](/docs/ref-varnish-cfg.md).

* The tool
  [``gogitversion``](https://github.com/slimhazard/gogitversion) to
  generate a version string using ``git describe``.

``gogitversion`` needs to be installed by hand; this sequence
suffices:

```
$ go get -d github.com/slimhazard/gogitversion
$ cd $GOPATH/src/github.com/slimhazard/gogitversion
$ make install
```

The k8s API code generators must be installed in the correct version
necessary for code compatibility as defined above; this is handled by
a Makefile target, discussed below.

Documentation for the k8s API generators is notoriously poor; what's
important to know for this project:

Code that must be written by hand before generation defines the Go
types that correspond to the Resources deployed in a cluster, and is
defined in the ``pkg/apis/`` folder in these sources (for version
``v1alpha1``):

```
pkg/apis/
├── register.go
└── varnishingress
    ├── register.go
    └── v1alpha1
        ├── doc.go
        ├── register.go
        └── types.go
```
The most important of these is ``types.go``, in which the Go types are
defined -- for example, this is where the ``VarnishConfig`` struct is
defined that encapsulates the ``VarnishConfig`` Custom Resource. Most
further development is likely to take place in that source, to update
the type or to add new types. At some point, of course, new versions
next to ``v1alpha1`` are likely to be added.

Code generation is driven by the ``+k8s``, ``+genclient`` and
``+groupname`` directive in the sources. The commands executed by the
``generate`` target of the Makefile (see below) are sufficient to
generate the client code. This is only necessary when types or the
version change, and won't need to be done for most builds. The
generated code is checked into the repo, and should not be edited, or
changed unless such a change is necessary.

The generated code is created in these package paths:

* ``pkg/client/clientset``: client code to access the types and interact
  with the k8s server API

* ``pkg/client/informers``: code for watching the API for updates
  involving the Custom Resources

* ``pkg/client/listers``: code for retrieving and listing values from
  the [client-go cache package](https://godoc.org/k8s.io/client-go/tools/cache)

There is no automated relation between the Go types and the
[Custom Resource definition](/docs/varnishcfg-crd.yaml), or any element
of configuration manifests. The correspondence must be established
with the ``json`` annotations used for structs and fields in ``types.go``.
The annotation MUST name fields used in the configuration manifests,
otherwise the client APIs will not correctly retrieve data that was
written to configure the cluster.

## Makefile

Targets in the Makefile:

* ``vgo``: runs ``go get`` for ``vgo``

* ``install-code-gen``: installs the k8s API code generators at the
  versions needed for compatibility with Kubernetes 1.9 (client-go 6.0.0)

* ``generate``: run the k8s API code generators. Since this is only
  done occasionally, the target is *not* a dependency for any other
  target; run only when needed, for example when types in ``types.go``
  have been updated, or when a new API version is introduced.

* ``build``: runs ``vgo generate`` (to run ``gogitversion``) ``vgo fmt``,
  and build the code in ``pkg/`` and ``cmd/``. The executable is *not*
  built.

* ``k8s-ingress``: runs the ``build`` target, and builds the
  controller executable.

* ``check``, ``test``: build the ``k8s-ingress`` executable if
  necessary, and run ``golint`` and ``vgo test``.

* ``clean``: run ``vgo clean``, and clean up other generated artifacts
