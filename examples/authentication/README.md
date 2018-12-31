# Basic and Proxy Authentication

This folder contains examples of configuration for Basic and Proxy
Authentication, as specified in the
[``.spec.auth`` section](/docs/ref-varnish-cfg.md) of a VarnishConfig
Custom Resource. See [RFC7235](https://tools.ietf.org/html/rfc7235)
for the HTTP Authentication standard.

The sample manifests are based on the
["cafe" example](/examples/hello), and pre-suppose that the Services
and Ingress from that example are deployed.

## Basic Authentication

The example for Basic Authentication requires authentication against
separate realms, with separate sets of credentials, for attempts to
access the "coffee" or "tea" services. In both cases, authentication
is required when the ``Host`` is ``cafe.example.com`` (which the
Ingress rules require for both services); but the separated
authentication rules apply for the URL paths ``/coffee`` and ``/tea``
(which the Ingress rules route to ``coffee-svc`` and ``tea-svc``).

First define the Secrets ``coffee-creds`` and ``tea-creds``, which
contain the user/password credentials for the two Services:

```
$ kubectl apply -f basic-secrets.yaml
```

The key-value pairs in the ``data`` section of each Secret form the
username-password pairs to be used for authentication (note that
``stringData`` is used in the YAMLs for convenience in these examples,
so that the passwords are human-readable):

```
apiVersion: v1
kind: Secret
metadata:
  name: coffee-creds
  labels:
    app: varnish-ingress
type: Opaque
stringData:
  coffee-admin: superpowers
  foo: bar
  baz: quux
  Aladdin: open sesame
```

Note that the Secret has the label ``app: varnish-ingress``. The
Ingress controller ignores all Secrets that do not have this label.

Now apply the ``VarnishConfig`` Custom Resource that defines the
configuration in the ``.spec.auth`` section:

```
$ kubectl apply -f basic-auth.yaml
```

The configuration references the Secrets in the ``secretName`` field;
these Secrets must exist in the same namespace as the VarnishConfig
resource, Ingress and Varnish Service. For the "coffee" service, we
set the authentication realm to ``coffee`` and specify credentials
from the ``coffee-creds`` Secret:

```
  auth:
    - realm: coffee
      secretName: coffee-creds
      type: basic
      utf8: true
      condition:
        host-match: ^cafe\.example\.com$
        url-match: ^/coffee($|/)
```

``type: basic`` specifies Basic Authentication, and the ``host-match``
and ``url-match`` fields require authentication in the "coffee" realm
when the Host is exactly equal to "cafe.example.com", and the URL path
begins with "/coffee".

The ``utf8: true`` setting means that the field ``charset="UTF-8"``
field is appended to the ``WWW-Authenticate`` response header when
authentication fails, to advise clients that UTF-8 encoding is used
for usernames and passwords (see
[RFC 7617 2.1](https://tools.ietf.org/html/rfc7617#section-2.1)).
This is not strictly necessary for the example, but you may need it if
credentials include characters outside of the ASCII range.

For the "tea" service, the realm is "tea" and credentials are taken
from the ``tea-creds`` Secret when the URL path begins with "/tea":

```
    - realm: tea
      secretName: tea-creds
      condition:
        host-match: ^cafe\.example\.com$
        url-match: ^/tea($|/)
```

Not that ``type: basic`` was left out here, since ``basic`` is the
default.

To verify the configuration after invoking the two ``kubectl``
commands: as with the ["cafe" example](/examples/hello), assume that
``$ADDR`` is the the external address of the Kubernetes cluster, and
that ``$PORT`` is the external port for which requests are received by
Varnish Services implementing Ingress.

For requests without credentials, responses with status
``401 Unauthorized`` are received with the ``WWW-Authenticate`` header
specifying the realm, and also the ``charset`` field in the case of
"/coffee":

```
$ curl -v -H 'Host: cafe.example.com' http://$ADDR:$PORT/coffee
[...]
< HTTP/1.1 401 Unauthorized
[...]
< WWW-Authenticate: Basic realm="coffee", charset="UTF-8"
[...]

$ curl -v -H 'Host: cafe.example.com' http://$ADDR:$PORT/tea
[...]
< HTTP/1.1 401 Unauthorized
[...]
< WWW-Authenticate: Basic realm="tea"
[...]
```

Requests with credentials from the respective Secret succeed:

```
$ curl --user foo:bar -v -H 'Host: cafe.example.com' http://$ADDR:$PORT/coffee
[...]
< HTTP/1.1 200 OK
[...]

$ curl --user tea-admin:awesomeness -v -H 'Host: cafe.example.com' http:/$ADDR:$PORT//tea
[...]
< HTTP/1.1 200 OK
[...]
```

Requests that are not routed to either Service according to the
Ingress rules get the 404 Not Found response as before, without
requiring authentication:

```
$ curl -v -H 'Host: cafe.example.com' http://$ADDR:$PORT/milk
[...]
< HTTP/1.1 404 Not Found
[...]
```

## Proxy Authentication

For the example of Proxy Authentication, we apply another Secret named
``proxy-creds``, and a VarnishConfig resource that specifies
``type:proxy`` in the ``auth`` element:

```
$ kubectl apply -f proxy-auth-secrets.yaml
$ kubectl apply -f proxy-auth.yaml
```

This configuration sets the realm to "ingress", and applies
unconditionally to all requests:

```
  auth:
    - realm: ingress
      secretName: proxy-creds
      type: proxy
```

As with Basic Authentication, it is also possible to use the
``condition.host-match`` and ``condition.url-match`` fields to
restrict the requests for which the authentication is required (but
Proxy Authentication typically applies to all requests).

To verify with curl, we use the ``-x`` (or ``--proxy``) argument to
specify ``$ADDR:$PORT`` as the proxy, and send the request with an
ordinary URI for the example domain. Credentials for a proxy are
supplied with the ``--proxy-user`` argument:

```
# Request without credentials for proxy authentication:
$ curl -v -x $ADDR:$PORT http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]

< HTTP/1.1 407 Proxy Authentication Required
[...]
< Proxy-Authenticate: Basic realm="ingress"

# With credentials:
$ curl --proxy-user proxy-admin:studly -v -x $ADDR:$PORT http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
> Proxy-Authorization: Basic cHJveHktYWRtaW46c3R1ZGx5
[...]

< HTTP/1.1 200 OK
[...]
```
