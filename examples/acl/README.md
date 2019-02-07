# Access Control Lists

The sample manifest in this folder sets values in the ``acl`` section
of a VarnishConfig to specify the whitelisting or blacklisting of IP
addresses againt access control lists. See the
[docs](/docs/ref-varnish-cfg.md) for the specifcation of ``acl`` in
VarnishConfig.

The example applies to the Ingress and Services defined in the
["cafe" example](/examples/hello).

Note that the order of elements in the ``acl`` array is significant.
If you define more than one ACL, then matches against the ACLs will be
executed in the order given in ``acl``, until one of them invokes the
failure status, or all of them pass.

## Whitelist for all client requests

The first example is a whitelist against the address ranges in private
IPv4 networks:

```
    - name: private-ip4
      addrs:
      - addr: 10.0.0.0
        mask-bits: 24
      - addr: 172.16.0.0
        mask-bits: 12
      - addr: 192.168.0.0
        mask-bits: 16
```

This sets the address ranges to:

* 10.0.0.0/24
* 172.16.0.0/12
* 192.168.0.0/16

The ``fail-status`` field is left out and has the default value 403,
meaning that a synthetic "403 Forbidden" response is generated for ACL
failures.  The ``type`` field has the default value ``whitelist``,
which means that the failure status is invoked for IP addresses that
do not match the ACL.

The ``comparand`` has the default value ``client.ip``. This means that
the ACL is matched against the VCL
[``client.ip`` object](https://varnish-cache.org/docs/6.1/reference/vcl.html#local-server-remote-and-client),
which is the client address sent in the PROXY header if the PROXY
protocol is in use, or the peer address of the connection if not.
(See the [docs](/docs/varnish-pod-template.md) about how to use the
PROXY protocol.) If ``comparand`` is set to any of ``server.ip``,
``remote.ip`` or ``local.ip``, then the IP address to be matched is
also evaluated as in VCL -- see the
[docs](https://varnish-cache.org/docs/6.1/reference/vcl.html#local-server-remote-and-client)
for details.

The ``conditions`` field is not set in this example, so the ACL match
is executed for every client request.

## ACL restricted to requests for a Service

The next example re-creates the ACL shown as an example in
[vcl(7)](https://varnish-cache.org/docs/6.1/reference/vcl.html#access-control-list-acl):

```
    - name: man-vcl-example
      addrs:
      - addr: localhost
      - addr: 192.0.2.0
        mask-bits: 24
      - addr: 192.0.2.23
        negate: true
      comparand: req.http.X-Real-IP
      type: whitelist
      conditions:
      - comparand: req.url
        compare: match
        value: ^/tea(/|$)
      - comparand: req.http.Host
        value: cafe.example.com
      result-header:
        header: req.http.X-Tea-Whitelisted
        success: "true"
        failure: "false"
```

Addresses that match the ACL are:

* the IP resolved for ``localhost`` at VCL load time
* the range 192.0.2.0/24

But the ACL does not match the IP 192.0.2.23, since ``negate`` is
``true`` for that value.

Note that if a host name is given (``localhost`` in the example), then
Varnish resolves it once at VCL load time, and uses the first IP that
it finds. The IP is never changed after that, so this is not useful
for dynamic DNS entries. ACL matches are always matches against fixed
sets of IP addresses.

The ``type`` is ``whitelist`` as in the first example, this time set
explicitly. The ``fail-status`` is again default 403.

The ``comparand`` is ``req.http.X-Real-IP``, meaning that the ACL is
matched against the IP address in the client request header
``X-Real-IP``.  The header must be present in the request and must
contain an IP address, or the ACL match will fail. This can be used in
a setup where Varnish receives requests from a component that sets the
client IP in a header like ``X-Real-IP``.

The ``conditions`` specify that the ACL match is executed when:

* the URL path begins with "/tea"
* the Host header is exactly "cafe.example.com"
    * The ``compare`` field is left out of the condition for
      ``req.http.Host``, so it defaults to the value ``equal``,
      meaning compare for string equality.

According to the Ingress in the ["cafe" example](/examples/hello),
requests are routed to the Service ``tea-svc`` under these
conditions. So ``conditions`` serves to restrict the ACL match to
requests for that Service.

The example also shows the use of the ``result-header`` field to
assign a value to a client request header, depending on the result of
the ACL match. In this case, the client request header
``X-Tea-Whitelisted`` is set to the string "true" if the address from
``X-Real-IP`` matches the ACL -- the string from the ``success`` field
is set when the failure status is not invoked, which in the case of a
whitelist means that the address under consideration matches the
ACL. If the address from ``X-Real-IP`` does not match the ACL, and
hence for a whitelist leads to the failure response, then the string
"false" from the ``failure`` field is assigned to
``X-Tea-Whitelisted``.

The use of ``request-header`` makes it possible to implement logic in
further request processing that depends on the ACL result. In this
case, the request header can be inspected for the result of
whitelisting. If the ``request-header`` field does not appear in the
ACL config, then no header is set as the result of the ACL match.

To verify this configuration with curl, we use the ``-x`` option (or
``--proxy``) set to ``$ADDR:$PORT``, where ``$ADDR`` is the external
address of the cluster, and ``$PORT`` is the port at which requests
are routed to the Ingress. We also use ``varnishlog`` on the Pods
implementing the Ingress to verify that the ``X-Tea-Whitelisted``
is set according to the ``result-header`` config.

```
# Requests without an X-Real-IP header fail the ACL match, and get
# the 403 Forbidden response
$ curl -v -x $ADDR:$PORT http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]

< HTTP/1.1 403 Forbidden
[...]

# varnishlog shows that X-Tea-Whitelisted was set to false:
*   << Request  >> 33494     
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /tea
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Tea-Whitelisted: false
[...]


# Request with the X-Real-IP header, but set to an IP that does not
# match the ACL:
$ curl -H 'X-Real-IP: 198.51.100.47' -v -x $ADDR:$PORT http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> X-Real-IP: 198.51.100.47
[...]

< HTTP/1.1 403 Forbidden
[...]

# varnishlog:
*   << Request  >> 66076     
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /tea
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Real-IP: 198.51.100.47
[...]
-   ReqHeader      X-Tea-Whitelisted: false
[...]


# Request with an X-Real-IP header that matches the ACL:
$ curl -H 'X-Real-IP: 192.0.2.120' -v -x $ADDR:$PORT http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> X-Real-IP: 192.0.2.120
[...]

< HTTP/1.1 200 OK
[...]

# varnishlog shows that X-Tea-Whitelisted was set to true:
*   << Request  >> 33592     
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /tea
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Real-IP: 192.0.2.120
[...]
-   ReqHeader      X-Tea-Whitelisted: true
[...]
```

## Blacklist and use of X-Forwarded-For

The next example defines a blacklist for the ranges 192.0.20/24 and
198.51.100.0/24, to match against the ``X-Forwarded-For`` header:

```
    - name: xff-first-example
      addrs:
      - addr: 192.0.2.0
        mask-bits: 24
      - addr: 198.51.100.0
        mask-bits: 24
      comparand: xff-first
      type: blacklist
      fail-status: 404
      conditions:
      - comparand: req.url
        compare: match
        value: ^/coffee/black(/|$)
      - comparand: req.http.Host
        compare: equal
        value: cafe.example.com
      result-header:
        header: req.http.X-Coffee-Blacklist
        failure: "true"
        success: "false"
```

Type ``blacklist`` means that the failure status is returned for IPs
that match the ACL.

``fail-status`` in this case is set to 404 for the "404 Not Found"
response, so clients who do not match the ACL get responses that
appear as if they used an invalid URL.

The ``comparand`` is set to ``xff-first``, which means that the ACL is
matched against the IP in the first comma-separated field of the
``X-Forwarded-For`` request header. If that field is not an IP
address, then the match fails. Note that if there is no
``X-Forwarded-For`` header in the request received by Varnish, Varnish
adds it with the value of ``client.ip``, before the ACL match is
evaluated; so ``xff-first`` is the same as matching against
``client.ip`` in that case.

Since ``X-Forwarded-For`` may appear more than once in a request
header, all instances of the header are consolidated into one header
before the match is performed, comma-separated in the order in which
they appeared.

The ``conditions`` specify that the ACL match is executed when:

* the URL path begins with "/coffee/black"
* the Host header is exactly "cafe.example.com"

Note that this ACL specification, which is restricted to
"/coffee/black" URLs, appears in the VarnishConfig before the next
one, which restricts ACL matches to URLs beginning with
"/coffee". This is important to the logic of ACL matching -- the match
for the "more specific" URL range is executed first.

The example also shows that the sense of setting the ``result-header``
is reversed for blacklisting. In this case, the client request header
``X-Coffee-Blacklist`` is set to the string "true" if the address from
``X-Forwarded-For`` matches the ACL, since the string from the
``failure`` field is set when the failure status is invoked. For
blacklists, this means that the address under consideration matches
the ACL. If the address from ``X-Forwarded-For`` does not match the
ACL, and hence does not lead to the failure response due to
blacklisting, then the string "false" from the ``success`` field is
assigned to ``X-Coffee-Blacklist``.

Verifying the ACL with curl:

```
# A request without any X-Forwarded-For header does not match the ACL,
# and hence is not blocked by the blacklist. Varnish adds
# X-Forwarded-For with the client IP before the match is evaluated,
# but that IP does not match the ACL.
$ curl -v -x $ADDR:$ADDR http://cafe.example.com/coffee/black
[...]
> GET http://cafe.example.com/coffee/black HTTP/1.1
> Host: cafe.example.com
[...]

< HTTP/1.1 200 OK
[...]

# varnishlog shows the X-Coffee-Blacklist was set to "false":
*   << Request  >> 33704     
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /coffee/black
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Coffee-Blacklist: false
[...]


# A request in which the first field of X-Forwarded-For does not match
# the blacklist is not blocked:
$ curl -H 'X-Forwarded-For: 203.0.113.47, 192.0.2.11' -v -x $ADDR:$PORT http://cafe.example.com/coffee/black
[...]
> GET http://cafe.example.com/coffee/black HTTP/1.1
> Host: cafe.example.com
[...]
> X-Forwarded-For: 203.0.113.47, 192.0.2.11
[...]

< HTTP/1.1 200 OK
[...]

# varnishlog:
*   << Request  >> 33741     
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /coffee/black
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Forwarded-For: 203.0.113.47, 192.0.2.11
[...]
-   ReqHeader      X-Coffee-Blacklist: false
[...]


# A request in which the first field of X-Forwarded-For matches the
# blacklist is blocked, and the client receives the 404 response
# as specified by fail-status:
$ curl -H 'X-Forwarded-For: 192.0.2.11' -v -x $ADDR:$PORT http://cafe.example.com/coffee/black
[...]
> GET http://cafe.example.com/coffee/black HTTP/1.1
> Host: cafe.example.com
[...]
> X-Forwarded-For: 192.0.2.11
[...]

< HTTP/1.1 404 Not Found
[...]

# varnishlog shows the X-Coffee-Blacklist was set to "true":
*   << Request  >> 163884    
[...]
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqMethod      GET
-   ReqURL         /coffee/black
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      X-Forwarded-For: 192.0.2.11
[...]
-   ReqHeader      X-Coffee-Blacklist: true
[...]
```

## Another use of X-Forwarded-For

The final example defines a blacklist for the range 203.0.113.0/24:

```
    - name: xff-2ndlast-example
      addrs:
      - addr: 203.0.113.0
        mask-bits: 24
      comparand: xff-2ndlast
      type: blacklist
      conditions:
      - comparand: req.url
        compare: match
        value: ^/coffee(/|$)
      - comparand: req.http.Host
        value: cafe.example.com
```

The ``comparand`` is ``xff-2ndlast``, which means that the ACL is
matched against the IP in the next-to-last comma-separated field of
the ``X-Forwarded-For`` request header, *after* Varnish appends the
client IP to ``X-Forwarded-For``. In other words, it specifies the IP
in the last field of ``X-Forwarded-For`` as received by Varnish.

For example, Varnish may receive a request in which the header looks
like this:

```
X-Forwarded-For: 192.0.2.47, 203.0.113.11
```

Varnish always appends the client IP, which may change the header to:

```
X-Forwarded-For: 192.0.2.47, 203.0.113.11, 172.17.0.1
```

``xff-2ndlast`` in this case specifies 203.0.113.11, which matches the
blacklist. If there is no ``X-Forwarded-For`` in a request, Varnish
adds the header with the client IP value; but then there is no
next-to-last field, so matches for ``xff-2ndlast`` fail.

The ``conditions`` specify that the ACL match is executed when:

* the URL path begins with "/coffee"
* the Host header is exactly "cafe.example.com"

In the ["cafe" example](/examples/hello), this applies to requests
that are routed to the Service ``coffee-svc``.

Verifying the blacklist:

```
# Request sent to Varnish in which the last field of X-Forwarded-For
# (which becomes next-to-last after Varnish modifies the header) does
# not match the blacklist, so it is not blocked:
$ curl -H 'X-Forwarded-For: 203.0.113.47, 192.0.2.11' -v -x $ADDR:$ADDR http://cafe.example.com/coffee/black
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> X-Forwarded-For: 203.0.113.47, 192.0.2.11
[...]

< HTTP/1.1 200 OK
[...]

# A request sent to Varnish in which the last (and only) field in
# X-Forwarded-For matches the blacklist is blocked:
$ curl -H 'X-Forwarded-For: 203.0.113.47' -v -x $ADDR:$PORT http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> X-Forwarded-For: 203.0.113.47
[...]

< HTTP/1.1 403 Forbidden
[...]

# A request sent with no X-Forwarded-For header never matches the
# blecklist, and hence is not blocked:
$ curl -v -x 192.168.0.100:30376 http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]

< HTTP/1.1 200 OK
[...]
```
