# Deploying an Ingress

Deployment of the Varnish Ingress in a Kubernetes cluster must fulfill
some requirements, while in some respects you can make choices
suitable to your requirements. The YAML configurations in this folder
prepare a method of deployment, suitable for testing and editing as
needed.

These instruction target a setup in which the Ingress controller runs
in the ``kube-system`` namespace, and watches for Varnish Ingresses in
all namespaces. If you need to restrict your deployment to a single
namespace, see the [instructions for single-namespace
deployments](/examples/namespace) in the [``/examples``
folder](/examples).

## The first time

The first steps must be executed for a new cluster, and will probably
be repeated only rarely afterward (for example for a software update).

### Containers

These containers must be availabe for pull in the Kubernetes cluster:

* Controller: ``varnish-ingress/controller``
* Varnish to implement Ingress: ``varnish-ingress/varnish``

See the [``container/`` folder](/container) for instructions for
building the containers.

### ServiceAccount and RBAC

Define a ServiceAccount named ``varnish-ingress-controller`` and apply
[Role-based access
control](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
(RBAC) to permit the necessary API access for the Ingress controller:
```
$ kubectl apply -f serviceaccount.yaml
$ kubectl apply -f rbac.yaml
```

### VarnishConfig Custom Resource definition

The project defines a
[Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
``VarnishConfig`` to specify special configurations and features of
Varnish running as an Ingress (beyond the standard Ingress
specification, see the [docs](/docs/ref-varnish-cfg.md) for details):

```
$ kubectl apply -f varnishcfg-crd.yaml
```

### BackendConfig Custom Resource definition

The project also defines the Custom Resource ``BackendConfig`` to
configure properties of Services that are implemented as Varnish
backends, such as timeouts, health probes and load-balancing (see
the [docs](/docs/ref-backend-cfg.md)):

```
$ kubectl apply -f backendcfg-crd.yaml
```

### Deploy the controller

This example uses a Deployment to run the controller container in the
``kube-system`` namespace:
```
$ kubectl apply -f controller.yaml
```
The ``image`` in the manifest must specify the controller (named
``varnish-ingress/controller`` above).

The manifest specifies a ``containerPort`` for HTTP, at which the
controller listens for the ``/metrics`` endpoint to publish
[metrics](ref-metrics.md) suitable for integration with
[Prometheus](https://prometheus.io/docs/introduction/overview/):

```
        ports:
        - name: http
          containerPort: 8080
```

The default value for the port number is 8080; to set a different
value, use the ``-metricsport`` [command-line option](ref-cli-options.md)
for the controller.

It does *not* make sense to deploy more than one replica of the
controller. If there are more controllers, all of them will connect to
the Varnish instances and send them the same administrative
commands. That is not an error (or there is a bug in the controller if
it does cause errors), but the extra work is superflous.

#### Controller options

Command-line options for the controller invocation can be set using the
``args`` section of the ``container`` specification:

```
      containers:
      - image: varnish-ingress/controller
        name: varnish-ingress-controller
        # [...]
        args:
        - -log-level=info
```

See the [command-line option reference](/docs/ref-cli-options.md) for
details.

## Deploying Varnish as an Ingress

These steps are executed for each namespace in which Varnish is to be
deployed as an Ingress implementation.

### Admin Secret

The controller uses Varnish's admin interface to manage Varnish
instances, which requires authorization using a shared secret. This is
prepared by defining a k8s Secret:
```
$ kubectl apply -f adm-secret.yaml
```

The ``metadata`` section MUST specify the label
``app: varnish-ingress``. The Ingress controller ignores all Secrets
that do not have this label.

The ``metadata.name`` field MUST match the secret name provided for
the Varnish deployment described below (``adm-secret`` in the
example):
```
metadata:
  name: adm-secret
  labels:
    app: varnish-ingress
```
32 bytes of randomness are sufficient for the secret:
```
# This command can be used to generate the value in the data field of
# the Secret:
$ head -c32 /dev/urandom | base64
```
**IMPORTANT**: Please do *not* copy the secret data from the sample
manifest in your deployment; create a new secret, for example using
the command shown above. The purpose of authorization is defeated if
everyone uses the same secret from an example.

**TO DO**: The key for the Secret (in the ``data`` field) is
hard-wired to ``admin``.

### Deploy Varnish containers

The present example uses a Deployment to deploy Varnish instances
(other possibilities are a DaemonSet or a StatefulSet):
```
$ kubectl apply -f varnish.yaml
```
With a choice such as a Deployment you can set as many replicas as you
need; the controller will manage all of them uniformly.

There are some requirements on the configuration of the Varnish
deployment that must be fulfilled in order for the Ingress to work
properly:

* The ``image`` must specify the Varnish container (named
  ``varnish-ingress/varnish`` above)
* ``spec.template`` must specify the label ``app: varnish-ingress``.
  The controller recognizes Services with this label as Varnish
  deployments meant to implement Ingress:

```
  template:
    metadata:
      labels:
        app: varnish-ingress
```

* The HTTP, readiness and admin ports must be specified:

```
        ports:
        - name: http
          containerPort: 80
        - name: k8sport
          containerPort: 8080
        - name: admport
          containerPort: 6081
```

  A port for TLS access is currently not supported.
* ``volumeMounts`` and ``volumes`` must be specified so that the
  Secret defined above is available to Varnish. The ``secretName``
  MUST match the name of the Secret (``adm-secret`` in the example):

```
        volumeMounts:
        - name: adm-secret
          mountPath: "/var/run/varnish"
          readOnly: true
```

```
      volumes:
      - name: adm-secret
        secret:
          secretName: adm-secret
          items:
          - key: admin
            path: _.secret
```
  **TO DO**: The ``key`` is hard-wired to ``admin``.

* The liveness check should determine if the Varnish master process is
  running. Since Varnish is started in the foreground as the entry
  point of the container, the container is live if it is running at
  all. This check verifies that a ``varnishd`` process with parent PID
  0 is found in the process table:

```
        livenessProbe:
          exec:
            command:
            - /usr/bin/pgrep
            - -P
            - "0"
            - varnishd
```

* The readiness check is an HTTP probe at the reserved listener (named
  ``k8sport`` above) for the URL path ``/ready``:

```
        readinessProbe:
          httpGet:
            path: /ready
            port: k8sport
```

  The port name must match the name given for port 8080 above.

The Deployment configuration in the current folder shows the default
Pod template for running Varnish, but it can be customized by setting
[varnishd command-line options](https://varnish-cache.org/docs/6.1/reference/varnishd.html#options)
in ``args`` and/or environment variables in ``env``. You may need to
do so, for example, to set the PROXY protocol for the HTTP listener,
change container port numbers, or configure Varnish tunables. See the
[documentation](/docs/varnish-pod-template.md) for details and requirements,
and the [``examples/`` folder](/examples/varnish_pod_template) for
working examples.

### Expose the Varnish HTTP and admin ports

With a Deployment, you may choose a resource such as a LoadBalancer or
Nodeport to create external access to Varnish's HTTP port. The present
example creates a Nodeport, which is simple for development and
testing:
```
$ kubectl apply -f nodeport.yaml
```
The cluster then assigns an external port over which HTTP requests are
directed to Varnish instances.

The Service definition must fulfill some requirements:

* A port with the name ``varnishadm`` and whose ``targetPort``
  matches the admin port defined above (named ``admport`` in the
  sample Deployment for Varnish) MUST be specified:

```
  ports:
  - port: 6081
    targetPort: 6081
    protocol: TCP
    name: varnishadm
```

  The external port for admin is not used; this allows the
  controller to identify the admin ports by name for the Endpoints
  that realize the Varnish Service.

  **TO DO**: The ``port.name`` is hardwired to ``varnishadm``.

* A port with the name ``http`` MUST be specified, whose
  ``targetPort`` matches the http port defined above (named ``http``
  in the sample Deployment).

  **TO DO**: The ``port.name`` is hardwired to ``http``.

* The Service must be defined so that the cluster API will allow
  Endpoints to be listed when the container is not ready (since
  the Varnish instances are initialized in the not ready state).
  The means for doing so has changed in different versions of
  Kubernetes. In versions up to 1.9, this annotation must be used:

```
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
```

  Since 1.9, the annotation is deprecated, and this field in ``spec``
  should be specified instead:

```
spec:
  publishNotReadyAddresses: true
```

  In recent versions, both specifications are permitted in the YAML,
  as in the example YAML (the annotation is deprecated, but is not yet
  an error).
* The ``selector`` must specify the label ``app: varnish-ingress``:

```
  selector:
    matchLabels:
      app: varnish-ingress
```

## Done

When these commands succeed:

* The Varnish instances are running and are in the not ready state.
  They answer with synthetic 503 Service Not Available responses to
  every request, for both readiness probes and regular HTTP traffic.
* The Ingress controller begins discovering Ingress definitions for
  the namespace of the Pod in which it is running (``varnish-ingress``
  in this example). Once it has obtained an Ingress definition, it
  creates a VCL configuration to implement it, and instructs the
  Varnish instances to load and use it.

You can now define Services that will serve as backends for the
Varnish instances, and Ingress rules that define how they route
requests to those Services.

The [``examples/``](/examples) folder of the repository contains
sample configurations for Services and Ingresses to test and
demonstrate the Varnish implementation. The
["cafe" example](/examples/hello), a kind of "hello world" for
Ingress, is a simple configuration that can be used to test
your deployment.
