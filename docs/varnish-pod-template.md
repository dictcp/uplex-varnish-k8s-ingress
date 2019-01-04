# Customizing the Varnish Pod template

The [deployment instructions](/deploy) include a
[sample manifest](/deploy/varnish.yaml) with a default configuration
of the Pod template for a Varnish instance, which can be used in the
configuration of a controller for Varnish (such as a Deployment,
DaemonSet, and so forth). The Pod template can be customized by
setting command-line arguments in the ``args`` section, and/or
environment variables in ``env``. You may want to do this in order to:

* require the
  [PROXY protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)
  for the Varnish listener

* set
  [Varnish runtime parameters and tunables](https://varnish-cache.org/docs/6.1/reference/varnishd.html)
  such as the cache size, default TTLs and timeouts, thread pool
  dimensions, workspace sizes (and many more)

* change the container configuration from default values, for example
  to set non-default port numbers, or a non-default mount path for the
  admin Secret

The [``examples/`` folder](/examples/varnish_pod_template) has
working examples that demonstrate such configurations. The present
document describes what may, and what may not be customized.

## varnishd Command-Line options

See
[``varnishd(1)``](https://varnish-cache.org/docs/6.1/reference/varnishd.html#options)
for details about available options.

```
        args:
        # varnishd command-line options
        # In this example:
        # varnishd -s malloc,256m -t 900 -p workspace_client=256k
        - -s
        - malloc,256m
        - -t
        - "900"
        - -p
        - workspace_client=256k
```

Because of the fact that the container starts with a number of options
in order to implement the role of an Ingress, there are restrictions
on the options that you can or should set. Some of them result in
illegal combinations of options, which causes varnishd to terminate
and the container to crash.  Others will be ignored, since the Varnish
instance is managed by the controller.  Still others may interfere
with the operation as an Ingress.

Among these restrictions are:

* You MAY NOT use any of the ``-C``, ``-d``, ``-I``, ``-S``, ``-T``,
  ``-V``, ``-x`` or ``-?`` options.

* The ``-p vcl_path`` parameter MAY NOT be changed.

* ``-b`` or ``-f`` SHOULD NOT be set, since they will be ignored (but
  their use does not cause an error).

* ``-a`` CAN be set to define more listener ports for regular HTTP
  client traffic; to be useful, these must be declared in the
  ``ports`` specification (as with the port named ``http`` in the
  pod template). The listener name ``vk8s`` MAY NOT be used (it is
  reserved for the listener used for readiness checks).

* ``-M`` CAN be set, so that Varnish will connect to an address
  listening for the administrative interface. The controller will not
  use that address, but an admin client can use it to monitor the
  Varnish instance separately. But an admin client MAY NOT call
  ``vcl.use`` to activate any configuration, or ``vcl.discard`` to
  unload one, otherwise it interferes with the implementation of
  Ingress.

See the [``examples/`` folder](/examples/varnish_pod_template) for a
[working manifest](/examples/varnish_pod_template/cli-args.yaml) that
sets command-line options.

## Environment variables

These environment variables can be used to change the configuration
from defaults:

* ``PROTO``: sets the
  [PROTO sub-argument](https://varnish-cache.org/docs/6.1/reference/varnishd.html#basic-options)
  for the HTTP listener. Legal values are ``HTTP`` or ``PROXY``,
  default ``HTTP``.

* ``HTTP_PORT``: sets the container port for the HTTP listener,
  default 80.

* ``READY_PORT``: sets the container port for the listener for
  readiness checks, default 8080.

* ``ADMIN_PORT``: sets the port at which Varnish listens for
  [CLI commands](https://varnish-cache.org/docs/6.1/reference/varnish-cli.html),
  used by the controller; default 6081.

* ``SECRET_PATH``: sets the path mounted to the volume that is
  populated with the admin Secret; default ``/var/run/varnish``.

* ``SECRET_FILE``: sets the basename of the file in which the admin
  Secret is stored, default ``_.secret``.

For example:

```
        env:
        # PROTO=PROXY causes the listener at the http port to accept
        # the PROXY protocol (v1 or v2).
        # see: https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
        - name: PROTO
          value: PROXY

        # Container port for the HTTP listener.
        # MUST match the value set for the http containerPort in the
        # Pod template, and the http targetPort for the Service.
        - name: HTTP_PORT
          value: "81"

        # Container port for the HTTP readiness check.
        # MUST match the value set for the k8s containerPort in the
        # Pod template.
        - name: READY_PORT
          value: "8000"
```

As indicated in the example, the values set for some of the
environment variables must match values in the configuration of other
elements:

* ``HTTP_PORT``: MUST match the value in the Pod template's
  ``.ports[n].containerPort`` field for the port number of the
  HTTP listener (named ``http`` in the
  [Deployment example](/deploy/varnish.yaml)). MUST also match the
  ``targetPort`` defined in the Service configuration for Varnish
  (also named ``http`` in the [Nodeport example](/deploy/nodeport.yaml)).

* ``READY_PORT``: MUST match ``.ports[n].containerPort`` for the port
  used in the http readiness check (``.readinessProbe.httpGet.port``).

* ``ADMIN_PORT``: MUST match ``.ports[n].containerPort`` for the admin
  listener (named ``varnishadm`` in the
  [Deployment example](/deploy/varnish.yaml)). MUST also match the
  ``targetPort`` defined in the Service configuration for Varnish
  (also ``varnishadm`` in the [Nodeport example](/deploy/nodeport.yaml)).

* ``SECRET_PATH``: MUST match the value of ``.volumeMounts.mountPath``
  in the Pod template's configuration of the volume mounted to inject
  the admin Secret.

* ``SECRET_FILE``: MUST match ``.volumes.secret.items[n].path`` in
  the Pod template's specfication of the file basename for the admin
  Secret.

See the [``examples/`` folder](/examples/varnish_pod_template) for a
[manifest](/examples/varnish_pod_template/proxy.yaml) that just turns
on the PROXY protocol, and
[another one](/examples/varnish_pod_template/env.yaml) that sets all
of the environment variables.
