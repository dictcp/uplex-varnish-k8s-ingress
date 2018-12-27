# Controller command-line options

The [controller executable](/docs/dev.md) ``k8s-ingress`` can
be started with command-line options, and these can be specified
in the [``args`` section](/deploy) of a [manifest](/deploy/controller.yaml)
that configures use of the container.

```
$ k8s-ingress --help
Usage of ./k8s-ingress:
  -alsologtostderr
    	log to standard error as well as files
  -kubeconfig string
    	config path for the cluster master URL, for out-of-cluster runs
  -log-level string
    	log level: one of PANIC, FATAL, ERROR, WARN, INFO, DEBUG, 
    	or TRACE (default "INFO")
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -masterurl string
    	cluster master URL, for out-of-cluster runs
  -namespace string
    	namespace in which to listen for resources (default all)
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -templatedir string
    	directory of templates for VCL generation. Defaults to 
    	the TEMPLATE_DIR env variable, if set, or the 
    	current working directory when the ingress 
    	controller is invoked
  -v value
    	log level for V logs
  -version
    	print version and exit
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
```

``-kubeconfig`` and ``-masterurl`` can be used to run the controller
out-of-cluster:

```
$ k8s-ingress --kubeconfig $HOME/.kube/config
$ k8s-ingress -masterurl=https://192.168.0.100:8443
```

Out-of-cluster runs are mainly useful for quick tests during
development, for example with minikube, to skip the steps of
re-building and re-deploying the container image. These options should
not be used to run the controller in-cluster.

``-namespace ns`` restricts the controller to the namespace ``ns`` --
it only watches for Ingresses, Services and so on in the given
namespace. This may be necessary, for example, to deploy the
controller in an environment in which you do not have the
authorization to set up [RBAC](/deploy) so that the controller can run
in ``kube-system`` and watch all namespaces. See the
[examples](/examples/namespace) for a full example of a
single-namespace configuration. The controller watches all namespaces
by default.

``-log-level`` sets the log level for the main controller code,
``INFO`` by default.

``-version`` prints the controller version and exits. ``-help`` prints
the usage message shown above and exits.

The remaining options exist because
[code generated for the client API](/docs/dev.md) import the
[glog](https://github.com/golang/glog) logger.

* ``-alsologtostderr``
* ``-log_backtrace_at``
* ``-log_dir``
* ``-logtostderr``
* ``-stderrthreshold``
* ``-v``
* ``-vmodule``

glog is in fact used minimally by the controller (only by the
generated code); logging is primarily controlled by the ``-log-level``
option.