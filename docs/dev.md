# Controller executable

This folder contains the source code for the controller executable.
The controller runs in its own container, and can manage groups of
Varnish instances.

## Development

The executable ``k8s-ingress`` is currently built with Go
1.10. Currently Kubernetes version 1.9 is supported, and the
controller has been also tested successfully with 1.10.

Builds are executed with the [``vgo``](https://github.com/golang/vgo)
tool, in anticipation of the
[modules](https://github.com/golang/go/wiki/Modules) feature for
dependency management that is experimental in Go 1.11, and expected to
be finalized in 1.12. ``vgo`` must be installed before you begin with
development:

```
$ go get -u golang.org/x/vgo
```

Targets in the Makefile support development in your local environment:

* ``k8s-ingress``: build the controller executable. This target also
  runs ``vgo generate`` (see below) and ``vgo fmt``.

* ``check``, ``test``: build the ``k8s-ingress`` executable if
  necessary, and run ``golint`` and ``go test``.

* ``clean``: run ``vgo clean``, and clean up other generated artifacts

The build currently depends on the tool
[``gogitversion``](https://github.com/slimhazard/gogitversion) for the
generate step, to generate a version string using ``git describe``,
which needs to be installed by hand. This sequence should suffice:

```
$ go get -d github.com/slimhazard/gogitversion
$ cd $GOPATH/src/github.com/slimhazard/gogitversion
$ make install
```
