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
      conditions:
        - comparand: req.http.Host
          value: cafe.example.com
          compare: equal
        - comparand: req.url
          value: ^/coffee($|/)
          compare: match
```

``type: basic`` specifies Basic Authentication, and the ``conditions``
array requires authentication when a request is routed to the
coffee-svc Service. The first element of ``comparand`` specifies that
the Host header is exactly equal to "cafe.example.com", and the second
specifies that the URL path begins with "/coffee". The Basic
Authentication protocol configured here is only executed when all of
the ``conditions`` are met.

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
      conditions:
        - comparand: req.http.Host
          value: cafe.example.com
          compare: equal
        - comparand: req.url
          value: ^/tea($|/)
          compare: match
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
``conditions`` array to restrict the requests for which the
authentication is required (but Proxy Authentication typically applies
to all requests).

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
# Authorization via either IP whitelisting or Basic Auth

The next example illustrates a technique for "either-or" authorization
-- access may be granted if an IP whitelist is matched; but if the
whitelist doesn't match, clients may be authorized via Basic
Authentication.

```
$ kubectl apply -f acl-or-auth.yaml
```

The "either-or" logic is brought about by the configuraton of the
``acl`` object (see the [``acl`` folder](/example/acl) for more
examples of ACL configs).  The config uses ``result-header`` to set a
client request header, based on the result of the ACL match. But it
sets ``fail-status`` to 0, which means that a failure response is not
generated on match failure; setting the request header is the only
effect of the match result. The ``auth`` configuration in turn is only
executed when that header has a specific value.

The ``acl`` config specifies an ACL match against the IP address in
the client request header ``X-Real-IP``:

```
  acl:
    - name: ip-whitelist
      addrs:
      - addr: 192.0.2.0
        mask-bits: 24
      - addr: 198.51.100.0
        mask-bits: 24
      - addr: 203.0.113.0
        mask-bits: 24
      comparand: req.http.X-Real-IP
      type: whitelist
      fail-status: 0
      result-header:
        header: req.http.X-Whitelisted
        success: "true"
        failure: "false"
```

If the ``X-Real-IP`` header matches the whitelist, then the client
request header ``X-Whitelisted`` is set to ``true``, otherwise
``false``. Since ``fail-status`` is set to 0, a failure reponse is not
generated when the whitelist match fails; request processing continues
with ``X-Whitelisted`` set to ``false``.

The ``auth`` configuration is only executed when ``X-Whitelisted`` is
set to ``false``:

```
  auth:
    - realm: cafe
      secretName: coffee-creds
      conditions:
        - comparand: req.http.X-Whitelisted
          value: "false"
          compare: equal
```

In that case, Basic Auth requires authentication against the
credentials in ``coffee-creds``.

Verification shows that the "either-or" logic is functional:

```
# An access attempt with neither of X-Real-IP or credentials for Basic Auth
# fails, and authentication for the realm "cafe" is requested.
$ curl -x $ADDR:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
[...]
>
< HTTP/1.1 401 Unauthorized
[...]
< WWW-Authenticate: Basic realm="cafe"
[...]

# The same result is obtained when X-Real-IP is set to an IP that does
# not match the whitelist:
$ curl -H 'X-Real-IP: 127.0.0.1' -x $ADDR:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
[...]
> X-Real-IP: 127.0.0.1
>
< HTTP/1.1 401 Unauthorized
[...]
< WWW-Authenticate: Basic realm="cafe"
[...]

# If X-Real-IP matches the whitelist, then access is granted and Basic
# Auth is not requested:
$ curl -H 'X-Real-IP: 192.0.2.1' -x $ADDR:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
[...]
> X-Real-IP: 192.0.2.1
>
< HTTP/1.1 200 OK
[...]

# If X-Real-IP is absent or does not match the whitelist, access can be
# granted via Basic Auth:
$ curl --user foo:bar -x $ADDR:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
[...]
> Authorization: Basic Zm9vOmJhcg==
[...]
>
< HTTP/1.1 200 OK
[...]

$ curl -H 'X-Real-IP: 127.0.0.1' --user foo:bar -x $ADDR:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
[...]
> Authorization: Basic Zm9vOmJhcg==
[...]
> X-Real-IP: 127.0.0.1
>
< HTTP/1.1 200 OK
[...]
```
