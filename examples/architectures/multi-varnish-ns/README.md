# Multiple Varnish Services in a namespace

The sample manifests in this folder implement this configuration:

* Two Varnish-as-Ingress Services in the same namespace.

* Two Ingress definitions use the
  ``ingress.varnish-cache.org/varnish-svc`` annotation to identify the
  two Varnish Services; so the Ingress rules are implemented
  separately by the Varnish Services.

The Ingresses all have the ``ingress.class:varnish`` annotation to
identify Varnish as the implementation of Ingress rules.

In this setup, it is necessary to use the ``varnish-svc`` Ingress
annotation to specify which Varnish Service implements the Ingress
rules. The annotation may be left out if there is only one Varnish
Service for Ingress in the entire cluster, or only one in the same
namespace with the Ingress definition. But if there are more, then the
annotation must specify which one executes the rules.

## The example

![multi Varnish per ns](multi-varnish-ns.png?raw=true "multiple Varnish Services per namespace")

The configuration is similar to the ["cafe" example](/examples/hello/)
in that it defines the Services ``coffee-svc`` and ``tea-svc``, and
Ingress rules route requests to those Services. There are also Varnish
Services ``varnish-coffee`` and ``varnish-tea`` in the same namespace.

* Ingress ``coffee-ingress`` defines the rule that requests with the
  Host ``coffee.example.com`` are routed to ``coffee-svc``. This
  Ingress has the ``varnish-svc`` annotation to specify the Varnish
  Service ``varnish-coffee`` as the one to implement its rules.

* Ingress ``tea-ingress`` defines the rule that requests with
  Host ``tea.example.com`` are routed to ``tea-svc``.  It uses
  the ``varnish-svc`` annotation to specify ``varnish-tea``.

## Deploying the example

First define the ``cafe`` namespace:

```
$ kubectl apply -f namespace.yaml
```

Then define the backend Deployments and Services. These are the same
simple applications used for the ["cafe" example](/examples/hello/),
but with ``namespace`` set to ``cafe``:

```
$ kubectl apply -f coffee.yaml
$ kubectl apply -f tea.yaml
```

Now define the two Varnish Service and associated resources. This is
similar to the sequence described in the [deployment
instructions](/deploy/), but we define for both Varnish deployments:

* a Secret, to authorize use of the Varnish admin interface

* the Varnish Service as a Nodeport (for simplicity's sake)

* a Deployment that specifies the ``varnish-ingress`` container, and
  some required properties for the Ingress implementation

Since there are two Varnish Services in the same namespace, the
Service and Deployment definitions use a label ``ingress`` with
separate values ``coffee`` and ``tea`` to keep them separated, and so
that the corresponding Services and Deployments select the same Pods.

```
$ kubectl apply -f adm-secret-tea.yaml
$ kubectl apply -f nodeport-tea.yaml
$ kubectl apply -f varnish-tea.yaml

$ kubectl apply -f adm-secret-coffee.yaml
$ kubectl apply -f nodeport-coffee.yaml
$ kubectl apply -f varnish-coffee.yaml
```

The routing rules to be implemented by Varnish can now be configured
by loading the Ingress definitions:

```
$ kubectl apply -f coffee-ingress.yaml
$ kubectl apply -f tea-ingress.yaml
```

## Verification

The log output of the Ingress controller shows the association of
Ingress definitions with Varnish Services:

```
Ingresses implemented by Varnish Service cafe/varnish-tea: [cafe/tea-ingress]

Ingresses implemented by Varnish Service cafe/varnish-coffee: [cafe/coffee-ingress]
```

The implementation of the Ingress rules by the Varnish Services can
now be verified, for example with curl. Since we are accessing the two
Varnish Services as Nodeports, they are accessed externally over two
different ports. In the following, we use:

* ``$IP_ADDR`` for the IP address of the Kubernetes cluster

* ``$IP_PORT_COFFEE`` for the port at which requests are forwarded to
  the Varnish Service ``varnish-coffee``

* ``$IP_PORT_TEA`` for the port at which requests are forwarded to
  Varnish Service ``varnish-tea``

These values are used with curl's ``-x`` option (or ``--proxy``), to
identify the IP/port address as a proxy.

```
# Requests sent to Varnish Service varnish-coffee with
# Host:coffee.example.com are routed to coffee-svc:
$ curl -v -x $IP_ADDR:$IP_PORT_COFFEE http://coffee.example.com/foo
[...]
> GET http://coffee.example.com/foo HTTP/1.1
> Host: coffee.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
Server name: coffee-6c47b9cb9c-vlvdz
[...]

# Requests sent to varnish-coffee with any other Host result in a 404
# response.
$ curl -v -x $IP_ADDR:$IP_PORT_COFFEE http://tea.example.com/foo
[...]
> GET http://tea.example.com/foo HTTP/1.1
> Host: tea.example.com
[...]
> 
< HTTP/1.1 404 Not Found
[...]

# Requests sent to Varnish Service varnish-tea with
# Host:tea.example.com are routed to tea-svc:
$ curl -v -x $IP_ADDR:$IP_PORT_TEA http://tea.example.com/bar
[...]
> GET http://tea.example.com/bar HTTP/1.1
> Host: tea.example.com
[...]
< HTTP/1.1 200 OK
[...]
Server name: tea-58d4697745-6z7v6
[...]

# Requests sent to varnish-tea with any other Host get the 404
# response:
$ curl -v -x $IP_ADDR:$IP_PORT_TEA http://coffee.example.com/bar
[...]
> GET http://coffee.example.com/bar HTTP/1.1
> Host: coffee.example.com
[...]
> 
< HTTP/1.1 404 Not Found
[...]
```
