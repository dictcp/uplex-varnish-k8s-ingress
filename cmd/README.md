# Controller executable

This folder contains the source code for the controller executable.
The controller runs in its own container, and can manage groups of
Varnish instances.

## Development

The executable ``k8s-ingress`` is currently built with Go 1.10.

Targets in the Makefile support development in your local environment,
and facilitate testing with ``minikube``:

* ``k8s-ingress``: build the controller executable. This target also
  runs ``go get`` for package dependencies, ``go generate`` (see
  below) and ``go fmt``.

* ``check``, ``test``: build the ``k8s-ingress`` executable if
  necessary, and run ``go vet``, ``golint`` and ``go test``.

* ``clean``: run ``go clean``, and clean up other generated artifacts

The build currently depends on the tool
[``gogitversion``](https://github.com/slimhazard/gogitversion) for the
generate step, to generate a version string using ``git describe``,
which needs to be installed by hand. This sequence should suffice:

```
$ go get -d github.com/slimhazard/gogitversion
$ cd $GOPATH/src/github.com/slimhazard/gogitversion
$ make install
```
