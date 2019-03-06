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
  -class string
	value of the Ingress annotation kubernetes.io/ingress.class
	the controller only considers Ingresses with this value for the
	annotation (default "varnish")
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
  -metricsport uint
	port at which to listen for the /metrics endpoint (default 8080)
  -monitorintvl duration
	interval at which the monitor thread checks and updates
	instances of Varnish that implement Ingress.
	Monitor deactivated when <= 0s (default 30s)
  -namespace string
    	namespace in which to listen for resources (default all)
  -readyfile string
	path of a file to touch when the controller is ready,
	for readiness probes
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

``-templatedir dir`` sets ``dir`` as the location for templates used
by the controller to generate VCL configurations. By default, the
controller uses the value of the environment variable
``TEMPLATE_DIR``, or the current working director if neither of the
command-line option nor the environment variable are set.

If ``-readyfile /path/to/file`` is set, then the controller removes
the file at that path immediately at startup, if any exists, and
touches it when it is ready. Readiness probes can then test the file
for existence. By default, no readiness file is created.

``-class ingclass`` sets the string ``ingclass`` (default ``varnish``)
as the required value of the Ingress annotation
``kubernetes.io/ingress.class``.  The controller ignores Ingresses
that do not have the annotation set to this value. This makes it
possible for the Varnish Ingress implementation to co-exist in a
cluster with other implementations, as long as the other
implementations also respect the annotation. It also makes it possible
to deploy more than one Varnish controller to manage Varnish Services
and Ingresses separately; see the
[documentation](/docs/ref-svcs-ingresses-ns.md) and
[examples](/examples/architectures/multi-controller/) for details.

``-monitorintvl`` sets the interval for the
[monitor](/docs/monitor.md). By default 30 seconds, and the monitor is
deactivated for values <= 0. The monitor sleeps this long between
monitor runs for Varnish Services. See the documentation at the link
for more details.

``-metricsport`` (default 8080) sets the port number at which the
controller listens for the HTTP endpoint ``/metrics`` to publish
[metrics](/docs/ref-metrics.md) that are suitable for integration with
[Prometheus](https://prometheus.io/docs/introduction/overview/). It
must match the value of the ``containerPort`` configured for ``http``
in the [Pod template](/deploy/controller.yaml) for the controller
(cf. the [deplyoment instructions](/deploy#deploy-the-controller)).

``-log-level`` sets the log level for the main controller code,
``INFO`` by default.

``-version`` prints the controller version and exits. ``-help`` prints
the usage message shown above and exits.

The remaining options exist because
[code generated for the client API](/docs/dev.md) imports the
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
