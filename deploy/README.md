# Deploying an Ingress

There is a variety of ways to deploy an Ingress in a Kubernetes
cluster. The YAML configurations in this folder prepare a simple
method of deployment, suitable for testing and editing according to
your needs.

## Namespace and ServiceAccount

Define the Namespace ``varnish-ingress``, and a ServiceAccount named
``varnish-ingress`` in that namespace:
```
$ kubectl apply -f ns-and-sa.yaml
```
**NOTE**: You can choose any Namespace, but currently all further
operations are restricted to that Namespace -- all resources described
in the following must be defined in the same Namespace. The controller
currently only reads information from the cluster API about Ingresses,
Services and so forth in the namespace of the pod in which it is
running; so all Varnish instances and every resource named in an
Ingress definition must defined in that namespace. This is likely to
become more flexible in future development.

## RBAC

Apply [Role-based access
control](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
(RBAC) by creating a ClusterRole named ``varnish-ingress`` that
permits the necessary API access for the Ingress controller, and a
ClusterRoleBinding that assigns the ClusterRole to the ServiceAccount
defined in the first step:
```
$ kubectl apply -f rbac.yaml
```

## Admin Secret

The controller uses Varnish's admin interface to manage the Varnish
instance, which requires authorization using a shared secret. This is
prepared by defining a k8s Secret:
```
$ kubectl apply -f adm-secret.yaml
```
32 bytes of randomness are sufficient for the secret:
```
# This command can be used to generate the value in the data field of
# the Secret:
$ head -c32 /dev/urandom | base64
```
**TO DO**: The ``metadata.name`` field of the Secret is currently
hard-wired to the value ``adm-secret``, and the key for the Secret (in
the ``data`` field) is hard-wired to ``admin``. The Secret must be
defined in the same Namespace defined above.

## Deploy Varnish containers

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

* Currently it must be defined in the same Namespace as defined
  above.
* The ``serviceAccountName`` must match the ServiceAccount defined
  above.
* The ``image`` must be specified as ``varnish-ingress/varnish``.
* ``spec.template`` must specify a ``label`` with a value that is
  matched by the Varnish admin Service described below. In this
  example:
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
**TO DO**: The ports are currently hard-wired to these port numbers.
A port for TLS access is currently not supported.
* ``volumeMounts`` and ``volumes`` must be specified so that the
  Secret defined above is available to Varnish:
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
**TO DO**: The ``mountPath`` is currently hard-wired to
``/var/run/varnish``.  The ``secretName`` is hard-wired to
``adm-secret``, the ``key`` to ``admin``, and ``path`` to
``_.secret``.
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

### varnishd options

Varnish command-line options can be specified using the ``args`` section
of the ``container`` specfication:

```
      containers:
      - image: varnish-ingress/varnish
        name: varnish-ingress
        # [...]
        args:
        # Starts varnishd with: -l 80M -p default_grace=10
        - -l
        - 80M
        - -p
        - default_grace=10
```

See
[``varnishd(1)``](https://varnish-cache.org/docs/6.1/reference/varnishd.html#options)
for details about the available options.

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
  ``ports`` specification (as with the port named ''http`` in the
  example above). The listener name ``vk8s`` MAY NOT be used (it is
  reserved for the listener used for readiness checks, declared as
  ``k8sport`` above).

* ``-M`` CAN be set, so that Varnish will connect to an address
  listening for the administrative interface. The controller will not
  use that address, but an admin client can use it to monitor the
  Varnish instance separately. But an admin client MAY NOT call
  ``vcl.use`` to activate any configuration, or ``vcl.discard`` to
  unload one, otherwise it interferes with the implementation of
  Ingress.

## Expose the Varnish HTTP port

With a Deployment, you may choose a resource such as a LoadBalancer or
Nodeport to create external access to Varnish's HTTP port. The present
example creates a Nodeport, which is simple for development and
testing (a LoadBalancer is more likely in production deployments):
```
$ kubectl apply -f nodeport.yaml
```
The cluster then assigns an external port over which HTTP requests are
directed to Varnish instances.

## Varnish admin Service

The controller discovers Varnish instances that it manages by
obtaining the Endpoints for a headless Service that the admin port:
```
$ kubectl apply -f varnish-adm-svc.yaml
```
This makes it possible for the controller to find the internal
addresses of Varnish instances and connect to their admin listeners.

The Service definition must fulfill some requirements:

* The Service must be defined so that the cluster API will allow
  Endpoints to be listed when the container is not ready (since
  the Varnish instances are initialized in the not ready state).
  The means for doing so has changed in different versions of
  Kubernetes. In versions up 1.9, this annotation must be used:
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
  as in example YAML (the annotation is deprecated, but is not yet an
  error).
* The ``selector`` must match the ``label`` given for the Varnish
  deployment, as discussed above. In the present example:
```
  selector:
    app: varnish-ingress
```

**TO DO**: The Service must be defined in the Namespace of the pod in
which the controller runs. The ``name`` of the Service is currently
hard-wired to ``varnish-ingress-admin``. The port number is hard-wired
to 6081, and the ``port.name`` is hardwired to ``varnishadm``.

## Deploy the controller

This example uses a Deployment to run the controller container:
```
$ kubectl apply -f controller.yaml
```
The requirements are:

* The ``image`` must be ``varnish-ingress/controller``.
* ``spec.template.spec`` must specify the ``POD_NAMESPACE``
  environment variable:
```
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

It does *not* make sense to deploy more than one replica of the
controller. If there are more controllers, all of them will connect to
the Varnish instances and send them the same administrative
commands. That is not an error (or there is a bug in the controller if
it does cause errors), but the extra work is superflous.

**TO DO**: The controller currently only acts on Ingress, Service,
Endpoint and Secret definitions in the same Namespace as the pod in
which it is running.

### Controller options

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

Currently supported options are:

* ``log-level`` to set the verbosity of logging. Possible values are
  ``panic``, ``fatal``, ``error``, ``warn``, ``info``, ``debug`` or
  ``trace``; default ``info``.

# Done

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

The [``examples/``](/examples) folder of the repository contains YAML
configurations for sample Services and an Ingress to test and
demonstrate the Ingress implementation.
