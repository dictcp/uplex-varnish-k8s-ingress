# A cluster-wide Varnish Service, and another in a namespace

The sample manifests in this folder implement the following
configuration in a cluster:

* A Varnish-as-Ingress deployment in the ``kube-system`` namespace
  acts as a "cluster-wide" service.

* But there is another Varnish deployment to implement one of the
  Ingresses in another namespace.

* Ingress definitions use the
  ``ingress.varnish-cache.org/varnish-svc`` annotation to identify the
  Varnish Service in ``kube-system`` as the one to implement their
  rules.

* The Ingress definition in the other namespace with a Varnish Service
  has no such annotation. The unique Varnish Service in the same
  namespace is assumed as the one to implement its rules.

The Ingresses all have the ``ingress.class:varnish`` annotation to
identify Varnish as the implementation of Ingress rules. Two Ingresses
are merged to form a set of rules implemented by the cluster-wide
Varnish Service.

The configuration illustrates these features:

* Use of the ``ingress.varnish-cache.org/varnish-svc`` in an Ingress
  definition to explicitly identify a Varnish Service to implement its
  rules.

* If an Ingress has no such annotation, and there is more than one
  Varnish Service for Ingress in the cluster, but exactly one in the
  same namespace, then the Service in the same namespace implements
  its rules.

* Merging Ingresses from different namespaces. A Varnish Service
  configures Services from different namespaces as backends, when
  Ingresses in the various namespaces reference backend Services in
  their own namespace.

## The example

![Varnish cluster-wide and per namespace](cluster-ns-wide.png?raw=true "Varnish cluster-wide and per namespace")

The configuration is similar to the ["cafe" example](/examples/hello/)
in that it defines "coffee" and "tea" Services, and Ingress rules
route requests to those Services. There is also an "other" Service to
serve as the default backend when no Ingress rules apply.

* In ``kube-system``, the Service ``varnish-ingress`` is deployed.
  The label ``app:varnish-ingress`` identifies it as an Ingress
  implementation to be managed by the controller defined for this
  project.

* In the ``cafe`` namespace, these resources are defined:

    * Services ``coffee-svc`` and ``tea-svc``

    * Service ``varnish-ingress``, with the ``app:varnish-ingress``
      label identifying it as an Ingress implementation.

    * Ingress ``tea-ingress``, defining the rule that requests with
      the Host ``tea.example.com`` are routed to ``tea-svc``. This
      Ingress has the ``varnish-svc`` annotation to specify the
      Varnish Service in ``kube-system`` as the one to implement its
      rules.

    * Ingress ``coffee-ingress``, with the rule that requests with
      Host ``coffee.example.com`` are routed to ``coffee-svc``.  There
      is no ``varnish-svc`` annotation. Since there is more than one
      Varinsh-as-Ingress Service in the cluster, but only one in
      namespace ``cafe``, the Varnish Service in the same namespace
      implements its rules.

* In the ``other`` namespace:

    * Service ``other-svc``

    * Ingress ``other-ingress``, in which ``other-svc`` is defined as
      a default backend (to which requests are routed when no other
      Ingress rules apply). Like ``tea-ingress`` discussed above, this
      Ingress uses the ``varnish-svc`` annotation to specify the
      Varnish Service in ``kube-system``.

The Varnish Ingress implementation combines these rules and routes
requests to the three Services.

## Deploying the example

First define the two namespaces:

```
$ kubectl apply -f namespace.yaml
```

Then define the backend Deployments and Services in the two
namespaces. These are the same simple applications used for the
["cafe" example](/examples/hello/), but with ``namespace``
configurations in their ``metadata``:

```
$ kubectl apply -f coffee.yaml
$ kubectl apply -f tea.yaml
$ kubectl apply -f other.yaml
```

Now define the two Varnish Service and associated resources in the
``kube-system`` and ``cafe`` namespaces. This is similar to the
sequence described in the [deployment instructions](/deploy/), but we
define for both Varnish deployments:

* a Secret, to authorize use of the Varnish admin interface

* the Varnish Service as a Nodeport (for simplicity's sake)

* a Deployment that specifies the ``varnish-ingress`` container, and
  some required properties for the Ingress implementation

```
# Set up the Varnish deployment in kube-system
$ kubectl apply -f adm-secret-system.yaml
$ kubectl apply -f nodeport-system.yaml
$ kubectl apply -f varnish-system.yaml

# And in namespace cafe
$ kubectl apply -f adm-secret-coffee.yaml
$ kubectl apply -f nodeport-coffee.yaml
$ kubectl apply -f varnish-coffee.yaml
```

The routing rules to be implemented by Varnish can now be configured
by loading the three Ingress definitions:

```
$ kubectl apply -f coffee-ingress.yaml
$ kubectl apply -f tea-ingress.yaml
$ kubectl apply -f other-ingress.yaml
```

## Verification

The log output of the Ingress controller shows the association of
Ingress definitions with Varnish Services:

```
Ingresses implemented by Varnish Service kube-system/varnish-ingress: [other/other-ingress cafe/tea-ingress]

Ingresses implemented by Varnish Service cafe/varnish-ingress: [cafe/coffee-ingress]
```

The implementation of the Ingress rules by the Varnish Services can
now be verified, for example with curl. Since we are accessing the two
Varnish Services as Nodeports, they are accessed externally over two
different ports. In the following, we use:

* ``$IP_ADDR`` for the IP address of the Kubernetes cluster

* ``$IP_PORT_SYSTEM`` for the port at which requests are forwarded to
  the Varnish Service in ``kube-system``

* ``$IP_PORT_CAFE`` for the port at which requests are forwarded to
  the Varnish Service in namespace ``cafe``

These values are used with curl's ``-x`` option (or ``--proxy``), to
identify the IP/port address as a proxy.

```
# Requests sent to the Varnish Service in kube-system with
# Host:tea.example.com are routed to tea-svc:
$ curl -v -x $IP_ADDR:$IP_PORT_SYSTEM http://tea.example.com/foo
[...]
> GET http://tea.example.com/foo HTTP/1.1
> Host: tea.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
Server name: tea-58d4697745-wxdzb
[...]

# Requests sent to the Varnish Service in kube-system with any other
# Host are routed to other-svc.
$ curl -v -x $IP_ADDR:$IP_PORT_SYSTEM http://anything.else/bar
[...]
> GET http://anything.else/bar HTTP/1.1
> Host: anything.else
[...]
> 
< HTTP/1.1 200 OK
[...]
Server name: other-55cfbbf569-hv7x2
[...]

# Requests sent to the Varnish Service in namespace cafe with
# Host:coffee.example.com are routed to tea-svc:
$ curl -v -x $IP_ADDR:$IP_PORT_CAFE http://coffee.example.com/coffee
[...]
> GET http://coffee.example.com/coffee HTTP/1.1
> Host: coffee.example.com
[...]
< HTTP/1.1 200 OK
[...]
Server name: coffee-6c47b9cb9c-vlvdz
[...]

# Requests sent to the Varnish Service in namespace cafe with
# any other Host get the 404 response:
$ curl -v -x $IP_ADDR:$IP_PORT_CAFE http://tea.example.com/foo
[...]
> GET http://tea.example.com/foo HTTP/1.1
> Host: tea.example.com
[...]
> 
< HTTP/1.1 404 Not Found
[...]
```
