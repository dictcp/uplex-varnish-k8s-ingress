# Prometheus metrics at the ``/metrics`` endpoint

This is the authoritative reference for metrics published by the
controller at the ``/metrics`` HTTP endpoint, suitable for integration
with the
[Prometheus toolkit](https://prometheus.io/docs/introduction/overview/).
Note that these are metrics about the controller, not about Varnish;
for that, consider using the
[Varnish exporter for Prometheus](https://github.com/jonnenauha/prometheus_varnish_exporter).

By default, the controller listens at port 8080 for the endpoint. The
port number can be changed by setting the ``containerPort`` for
``http`` in the [Pod template](/deploy/controller.yaml) for the
controller (cf. the
[deployment instructions](/deploy#deploy-the-controller)), and setting the
``-metricsport`` [command-line option](res-cli-options.md).

The metric names follow the pattern ``$NAMESPACE_$SUBSYSTEM_$NAME``,
where ``$NAMESPACE`` and ``$SUBSYSTEM`` are separated by the
underscores (but ``$NAME`` may include underscores). The namespace of
main interest for the Ingress controller is ``varnishingctl``, whose
metrics are detailed in the following.

The endpoint also publishes metrics from three other namespaces,
which are included automatically by the
[client library](https://github.com/prometheus/client_golang)
used to implement it:

* ``process_*``: process information such as memory, CPU and file
  descriptor usage (implemented by the
  [Process Collector](https://godoc.org/github.com/prometheus/client_golang/prometheus#NewProcessCollector))

* ``go_*``: metrics from the Go runtime, such as garbage collection,
  memory stats, and goroutines (implemented by the
  [Go Collector](https://godoc.org/github.com/prometheus/client_golang/prometheus#NewGoCollector))

* ``promhttp_*``: metrics about the handler for the ``/metrics``
  endpoint, such as request counts and response codes

These are not documented further here
([this article](https://povilasv.me/prometheus-go-metrics/) has
more information about the Process and Go collectors).

The subsystems in use for the ``varnishingctl`` namespace are:

* ``varnishingctl_sync_*``: metrics about the actions taken to synchronize
  the cluster with the desired state

* ``varnishingctl_varnish_*``: metrics about the Varnish instances managed
  by the controller

* ``varnishingctl_watcher_*``: metrics about event notifications from the
  watcher API (Adds, Deletes and Updates per namespace and resource type)

* ``varnishingctl_workqueue_*``: metrics about the queueing of work items
  obtained from the watcher API

## Overview

This table serves as a quick reference; see below for more details
about the metrics. The Summary types have standard ``*_sum`` and
``*_count`` forms that are not detailed further here.

| Name | Type | Help string | Labels |
| ---: | :--  | :---        | :---   |
| ``varnishingctl_sync_result_total`` | Counter | Total number of synchronization results | ``kind``<br/>``namespace``<br/>``result`` |
| ``varnishingctl_varnish_admin_connect_fails_total`` | Counter | Total number of admin connection failures | ``varnish_instance`` |
| ``varnishingctl_varnish_admin_connect_latency_seconds`` | Summary | Admin connection latency | ``varnish_instance``<br/>(quantiles) |
| ``varnishingctl_varnish_admin_connect_latency_seconds_sum`` | | | |
| ``varnishingctl_varnish_admin_connect_latency_seconds_count`` | | | |
| ``varnishingctl_varnish_backend_endpoints`` | Gauge | Current number of Services endpoints configured as Varnish backends | |
| ``varnishingctl_varnish_backend_services`` | Gauge | Current number of Services configured as Varnish backends | |
| ``varnishingctl_varnish_child_not_running_total`` | Counter |Total number of monitor runs with the child process not in the running state | ``varnish_instance`` |
| ``varnishingctl_varnish_child_running_total`` | Counter | Total number of monitor runs with the child process in the running state | ``varnish_instance`` |
| ``varnishingctl_varnish_instances`` | Gauge | Current number of managed Varnish instances | |
| ``varnishingctl_varnish_monitor_checks_total`` | Counter | Total number of monitor checks | ``varnish_instance`` |
| ``varnishingctl_varnish_panics_total`` | Counter | Total number of panics detected | ``varnish_instance`` |
| ``varnishingctl_varnish_ping_errors_total`` | Counter | Total number of ping errors | ``varnish_instance`` |
| ``varnishingctl_varnish_pings_total`` | Counter | Total number of successful pings | ``varnish_instance`` |
| ``varnishingctl_varnish_secrets`` | Gauge | Current number of known admin secrets | |
| ``varnishingctl_varnish_services`` | Gauge | Current number of managed Varnish services | |
| ``varnishingctl_varnish_update_errors_total`` | Counter | Total number of update errors | ``varnish_instance`` |
| ``varnishingctl_varnish_updates_total`` | Counter | Total number of attempted updates | ``varnish_instance`` |
| ``varnishingctl_varnish_vcl_discards_total`` | Counter | Total number of VCL discards | ``varnish_instance`` |
| ``varnishingctl_varnish_vcl_load_errors_total`` | Counter | Total number of VCL load errors | ``varnish_instance`` |
| ``varnishingctl_varnish_vcl_load_latency_seconds`` | Summary | VCL load latency | ``varnish_instance``<br/>(quantiles)
| ``varnishingctl_varnish_vcl_load_latency_seconds_sum`` | | | ``varnish_instance`` |
| ``varnishingctl_varnish_vcl_load_latency_seconds_count`` | | | ``varnish_instance`` |
| ``varnishingctl_varnish_vcl_loads_total`` | Counter | Total number of successful VCL loads | ``varnish_instance`` |
| ``varnishingctl_watcher_events_total`` | Counter | Total number of watcher API events | ``event``<br/>``kind`` |
| ``varnishingctl_workqueue_adds_total`` | Counter | Total number of adds handled by the workqueue | ``namespace`` |
| ``varnishingctl_workqueue_depth`` | Gauge | Current depth of the workqueue | ``namespace`` |
| ``varnishingctl_workqueue_latency_useconds`` | Summary | Time spent (in microseconds) by items waiting in the workqueue | ``namespace``<br/>(quantiles) |
| ``varnishingctl_workqueue_latency_useconds_sum`` | | | ``namespace`` |
| ``varnishingctl_workqueue_latency_useconds_count`` | | | ``namespace`` |
| ``varnishingctl_workqueue_retries_total`` | Counter | Total number of retries handled by workqueue | ``namespace`` |
| ``varnishingctl_workqueue_work_duration_useconds`` | Summary | Time needed (in microseconds) to process items from the workqueue | ``namespace``<br/>(quantiles) |
| ``varnishingctl_workqueue_work_duration_useconds_sum`` | | | ``namespace`` |
| ``varnishingctl_workqueue_work_duration_useconds_count`` | | | ``namespace`` |

## Subsystem ``varnishingctl_sync``

This group has only one counter metric
``varnishingctl_sync_result_total``, but it is differentiated by three
labels:

* ``kind`` -- kind of Kubernetes resource synchronized; one of:

    * ``BackendConfig``

    * ``Endpoints``

    * ``Ingress``

    * ``Secret``

    * ``Service``

    * ``VarnishConfig``

    * ``Unknown``: if the resource kind could not be determined (this
      is an error)

* ``namespace`` -- the resource's namespace

* ``result`` -- one of:

    * ``SyncSuccess``

    * ``SyncFailure``

    * ``Ignored``

Each counter is incremented when the controller's efforts to
synchronize the desired state of a resource of type ``kind`` in the
``namespace`` has the indicated ``result``.

The values ``SyncSuccess`` and ``SyncFailure`` for the ``result``
label have the same meaning as the reason string for the corresponding
[Event](/docs/monitor.md#events), and the counter is incremented with
those label values at the same time the Events are generated. The
counter is incremented for ``result=Ignored`` if the controller is not
responsible for a resource about which it was informed by the watcher
API. This includes Secrets that do not have the
``app:varnish-ingress`` label, or Services that neither implement
Varnish-as-Ingress, nor appear as a backend of any Ingress definition.

The metric is not created for every possible permutation of the three
labels, only when a counter takes on a value > 0. For example, unless
there is a bug in the controller, a counter with ``kind=Unknown`` will
never be created.

## Subsystem ``varnishingctl_varnish``

This is a group of metrics concerning the controller's remote
management and monitoring of Varnish instances.

* ``varnishingctl_varnish_admin_connect_fails_total``

* ``varnishingctl_varnish_admin_connect_latency_seconds``

These are statistics about the controller's connection to the admin
ports of Varnish instances; the values of the label
``varnish_instance`` are the addresses of the instances (endpoint IP
and admin port).  This is critical to the operation of the controller,
and should be monitored for failures or long latencies.

* ``varnishingctl_varnish_vcl_loads_total``

* ``varnishingctl_varnish_vcl_load_errors_total``

* ``varnishingctl_varnish_vcl_load_latency_seconds``

Statistics about remote VCL loads performed by the controller over the
admin connection. This is also a critical operation that should be
monitored for failures or long latencies. The latency statistic
encompasses all of the time needed to load VCL, including transfer of
VCL sources over the admin connection, and running the VCL compiler.

* ``varnishingctl_varnish_updates_total``

* ``varnishingctl_varnish_update_errors_total``

This is a counter for update operations taken as a whole, including
VCL loads and relabeling of VCL configurations. The ``errors`` counter
increments if any part of an update attempt fails.

* ``varnishingctl_varnish_services``

* ``varnishingctl_varnish_instances``

These are cluster-wide gauges concerning the Services running Varnish
to implement Ingress. For example, if you have two such Services in
your cluster, one of which as three replicas and the other has four,
then ``varnishingctl_varnish_services`` = 2 and
``varnishingctl_varnish_instances`` = 7.

* ``varnishingctl_varnish_backend_services``

* ``varnishingctl_varnish_backend_endpoints``

These are cluster-wide gauges concerning the Services in use as Ingress
backends in currently active configurations. For example, suppose the
["cafe" example](/examples/hello), with the ``tea-svc`` and ``coffee-svc``
as Ingress backends, is the only Ingress in use in the cluster; and
that the two Services have 2 and 3 endpoints, respectively. Then
``varnishingctl_varnish_backend_endpoints`` = 5 and
``varnishingctl_varnish_backend_services`` = 2

* ``varnishingctl_varnish_secrets``

The number of Secrets in use to authorize Varnish admin connections.

* ``varnishingctl_varnish_monitor_checks_total``

* ``varnishingctl_varnish_child_running_total``

* ``varnishingctl_varnish_child_not_running_total``

* ``varnishingctl_varnish_panics_total``

* ``varnishingctl_varnish_pings_total``

* ``varnishingctl_varnish_ping_errors_total``

* ``varnishingctl_varnish_vcl_discards_total``

These metrics report the work of the
[Varnish monitor](/docs/monitor.md).  The error conditions should be
monitored, and the "good" statistics should always increase at a
constant rate (every time the monitor runs); otherwise the monitor may
have stopped running (which would be a bug of the controller).

## Subsystem ``varnishingctl_watcher``

This group has only one counter metric
``varnishingctl_watcher_events_total``, differentiated by two labels:

* ``kind`` -- kind of Kubernetes resource synchronized; one of:

    * ``BackendConfig``

    * ``Endpoints``

    * ``Ingress``

    * ``Secret``

    * ``Service``

    * ``VarnishConfig``

    * ``Unknown``: if the resource kind could not be determined (an
      error)

* ``event`` -- one of:

    * ``Add``

    * ``Delete``

    * ``Update``

The metrics are incremented for every corresponding event received by
the watcher API -- each Add, Delete or Update for the corresponding
``kind``.

## Subsystem ``varnishingctl_workqueue``

These metrics are generated automatically by the
[client library](https://godoc.org/k8s.io/client-go/util/workqueue)
used by the controller to queue work items received by the watcher
API.

The metrics use the label ``namespace``, which reflects the fanout by
namespace implemented by the controller. All work items from the
watcher API are initially placed on one queue for all namespaces,
indicated by the label value ``namespace=_ALL_``. From there they are
distributed to a queue for every namespace in the cluster about which
the controller is informed; these are then read for synchronization
work in separate goroutines.

* ``varnishingctl_workqueue_adds_total``

Queue adds per namespace.

* ``varnishingctl_workqueue_depth``

The current depth (length) of each queue.

* ``varnishingctl_workqueue_latency_useconds``

The time (in microseconds) spent by work items waiting in each queue.

* ``varnishingctl_workqueue_retries_total``

Number of retries for work items in each queue; that is, how often the
same work item was placed back onto a queue, usually because of an
error condition.

* ``varnishingctl_work_duration_useconds``

The time (in microseconds) needed to process work items from each
queue.  This is a measure of how much time the controller needs to
perform synchronizations in the cluster.
