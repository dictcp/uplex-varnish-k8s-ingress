# Multiple controllers

The controller is designed so that in most deployments, it suffices to
run it in exactly one Pod in the cluster, to manage all Varnish
Services and Ingresses in the cluster. But more than one controller
instance can be run by following the method described in the
[documentation](/docs/ref-svcs-ingresses-ns.md). The sample manifests
in this folder demonstrate a working example.

Multiple controllers are only assured to work correctly if they manage
distinct sets of Varnish Services and Ingresses (otherwise the results
are undefined). This is accomplished by:

* Starting the different controller instances with different values of
  the [command-line option ``-class``](/docs/ref-cli-options.md),
  defining the value of the Ingress annotation
  ``kubernetes.io/ingress.class`` that the controller instance will
  consider. This defines which controllers manage which Ingresses.

* No two Ingress definitions with different values of the
  ``ingress.class`` annotation should designate the same Varnish
  Service to implement the Ingress rules (by the rules for determining
  the Varnish Service as described in the
  [documentation](/docs/ref-svcs-ingresses-ns.md)).

The [Deployment
manifest](/examples/architectures/multi-controller/controller.yaml)
for a Varnish controller in this folder shows the use of the ``-class``
option in the ``spec.args`` field of its Pod template:

```
        args:
        - -readyfile=/ready
        - -class=varnish-coffee
```

This sets the value of the ``ingress.class`` annotation for Ingresses
that the controller considers.

## The example

![multiple controllers](multi-controller.png?raw=true "multiple controllers")

The configuration is similar to the ["cafe" example](/examples/hello/)
in that it defines the Services ``coffee-svc`` and ``tea-svc``, and
Ingress rules route requests to those Services. There are also Varnish
Services ``varnish-coffee`` and ``varnish-tea`` in the same namespace.

* Controller instance ``varnish-ingress-controller`` is started with
  the default value ``"varnish"`` for the ``ingress.class``
  annotation.  This is the same configuration defined by the manifests
  in the [``deploy/`` folder](/deploy/); the configuration is not
  included in the present folder.

* Controller instance ``varnish-coffee-ingress-controller`` is started
  with the [command-line option ``-class``](/docs/ref-cli-options.md)
  set to ``"varnish-coffee"``, so that this instance only considers
  Ingresses with that value for the ``ingress.class`` annotation.

* Ingress ``tea-ingress`` sets the ``ingress.class`` annotation to
  ``"varnish"``. It defines the rule that requests with Host
  ``tea.example.com`` are routed to ``tea-svc``.  This Ingress has the
  ``varnish-svc`` annotation to specify the Varnish Service
  ``varnish-tea`` as the one to implement its rules.

* Ingress ``coffee-ingress`` sets the ``ingess.class`` annotation to
  defines the rule that requests with the Host ``coffee.example.com``
  are routed to ``coffee-svc``. It uses ``varnish-svc`` to specify the
  Varnish Service ``varnish-coffee``.

The effect is that:

* Controller ``varnish-ingress-controller`` manages Varnish Service
  ``varnish-tea`` to implement the Ingress rule in ``tea-ingress``.

* Controller ``varnish-coffee-ingress-controller`` manages Varnish
  Service ``varnish-coffee`` to implement the Ingress rule in
  ``coffee-ingress``.

## Deploying the example

First deploy the ``varnish-ingress-controller`` instance as described
in the [deployment instructions](/deploy/), and then deploy
``varnish-coffee-ingress-controller`` as the second controller
instance:

```
$ kubectl apply -f controller.yaml
```

Then define the ``cafe`` namespace:

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
done the same way as described for the [example of multiple Varnish
Services in a namespace](/examples/architectures/multi-varnish-ns/):

```
$ kubectl apply -f adm-secret-tea.yaml
$ kubectl apply -f nodeport-tea.yaml
$ kubectl apply -f varnish-tea.yaml

$ kubectl apply -f adm-secret-coffee.yaml
$ kubectl apply -f nodeport-coffee.yaml
$ kubectl apply -f varnish-coffee.yaml
```

(Running multiple controllers does not depend on whether or not multiple
Varnish Services are run in the same namespace.)

The routing rules to be implemented by Varnish can now be configured
by loading the Ingress definitions:

```
$ kubectl apply -f coffee-ingress.yaml
$ kubectl apply -f tea-ingress.yaml
```

## Verification

The log output of the two controller instance shows their use of
different values for ``ingress.class``, which in turn determines which
of the Ingresses they manage or ignore.

In the log output for ``varnish-ingress-controller``:

```
Ingress class:varnish

Ingress cafe/tea-ingress configured for Varnish Service cafe/varnish-tea

Ignoring Ingress cafe/coffee-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish'
```

In the log for ``varnish-coffee-ingress-controller``:

```
Ingress class:varnish-coffee

Ingress cafe/coffee-ingress configured for Varnish Service cafe/varnish-coffee

Ignoring Ingress cafe/tea-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish-coffee'
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
Server name: coffee-6c47b9cb9c-mgh48
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
Server name: tea-58d4697745-wxdzb
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
