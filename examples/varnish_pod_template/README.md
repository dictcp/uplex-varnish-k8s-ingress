# Customizing the Varnish Pod template

The sample manifests in this folder set values in the ``args`` or
``env`` section of the Pod template for Varnish, to modify the
configuration from defaults. See the
[``docs/`` folder](/docs/varnish-pod-template.md) for details and
requirements.

As in the [example](/deploy/varnish.yaml) from the
[deployment instructions](/deploy), each of these define a Deployment,
but the configuration does not depend on that -- Pod templates are
also used for other types of controllers, such as DaemonSets.

[``cli-args.yaml``](cli-args.yaml) sets varnishd command-line options:

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

See
[``varnishd(1)``](https://varnish-cache.org/docs/6.1/reference/varnishd.html#options)
for details.

[``proxy.yaml``](proxy.yaml) just sets the ``PROTO`` environment
variable to activate the
[PROXY protocol](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)
(likely a common use case):

```
        env:
        # PROTO=PROXY causes the listener at the http port to accept
        # the PROXY protocol (v1 or v2).
        - name: PROTO
          value: PROXY
```

[``env.yaml``](env.yaml) sets all of the relevant environment
variables.  It also defines the Service for Varnish, since the values
of variables that set port numbers (``HTTP_PORT`` and ``ADMIN_PORT``)
must match ``targetPort`` values set for the Service:

```
apiVersion: apps/v1
kind: Deployment
[...]
spec:
[...]
  template:
    [...]
    spec:
      containers:
      - image: varnish-ingress/varnish
        [...]
        env:
        [...]

        # Container port for the HTTP listener.
        # MUST match the http targetPort in the Service below.
        - name: HTTP_PORT
          value: "81"

        [...]

        # Container port for the admin listener.
        # MUST match the varnishadm targetPort in the Service below.
        - name: ADMIN_PORT
          value: "7000"
[...]
---
apiVersion: v1
kind: Service
[...]
spec:
  type: NodePort 
  ports:
  - port: 7000
    targetPort: 7000
    protocol: TCP
    name: varnishadm
  - port: 81
    targetPort: 81
    protocol: TCP
    name: http
[...]

```
