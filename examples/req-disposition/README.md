# Disposition of client requests

The sample manifests in this folder configures the disposition of
client requests -- how client requests are further processed after
request headers have been received. The ``req-disposition`` field of
the [``VarnishConfig`` custom resource](/docs/ref-varnish-cfg.md)
determines the next [processing
state](https://varnish-cache.org/docs/6.1/reference/states.html) for a
request, subject to properties of the request.  It overrides the
implementation of ``vcl_recv`` in
[``builtin.vcl``](https://github.com/varnishcache/varnish-cache/blob/6.1/bin/varnishd/builtin.vcl).

A common use case for ``req-disposition`` is to allow caching for
requests that use cookies or basic authentication, since the Varnish
default, implemented in ``builtin.vcl``, is to bypass the cache for
such requests. The default is the cautious approach, since the
responses to requests with cookies or basic auth may be
personalized. You may use ``req-disposition`` to relax the default,
but make sure that personalized responses are not cached as a result.

A variety of other features can be implemented with
``req-disposition``, such as:

* specifically invoking cache lookup or bypass based on properites of
  the client request, such as URL path patterns. In other words,
  defining cacheability based on properties of the client request.

* white- and blacklisting requests

* defining a means to use the
  [purge](https://varnish-cache.org/docs/6.1/users-guide/purging.html)
  facility via client requets, for example by defining a ``PURGE``
  request method.

Examples for such configurations are discussed below.

See the [docs](/docs/ref-req-disposition.md) for the technical reference
for the configuration of client request dispositions.

The examples apply to the Ingress and Services defined in the
["cafe" example](/examples/hello).

## Reconstructing built-in ``vcl_recv`` in YAML

The first example re-implements the implementation of ``vcl_recv`` in
``builtin.vcl`` as a ``req-disposition`` configuration. Of course it
doesn't make sense to deploy this configuration unchanged in your
cluster; since it does exactly what built-in ``vcl_recv`` does, you
might as well leave it out and let Varnish execute the built-in
version. But the YAML is presented here as an illustration of the
configuration, and to show which parts of the default policies you
might want to change.

Apply the configuration with:

```
$ kubectl apply -f builtin.yaml
```

``req-disposition`` is an array of objects with the fields
``conditions`` and ``disposition``. Each object is evaluated against a
client request in the order of the array; for the first one for which
the ``conditions`` evaluate as true, the corresponding ``disposition``
determines further processing of the request. If none of the
``conditions`` evaluate as true, the request proceeds to cache lookup.

The first element of ``req-disposition`` says that if the request
method is ``PRI`` (tested with case-sensitive string equality), then
send a synthetic response with status ``405 Method Not Allowed``:

```
    - conditions:
      - comparand: req.method
        compare: equal
        values:
          - PRI
      disposition:
        action: synth
        status: 405
```

The ``PRI`` method is part of the HTTP/2 connection preface, see
[RFC7540](https://tools.ietf.org/html/rfc7540#section-3.5).

The next element enforces the requirement of HTTP/1.1 that the
``Host`` header must be present in a request:

```
    - conditions:
      - comparand: req.http.Host
        compare: not-exists
      - comparand: req.esi_level
        count: 0
      - comparand: req.proto
        compare: prefix
        values:
          - HTTP/1.1
        match-flags:
          case-sensitive: false
      disposition:
        action: synth
        status: 400
```

These ``conditions`` are true if each of the following are true:

* The ``Host`` header is not present in the request
  (``compare:not.exists``).

* The request is not ESI-included -- ``req.esi_level`` is 0. Notice
  that the ``compare`` field is not specified in the second clause, so
  the default comparison ``equals`` is assumed.

* The protocol begins with the strings ``HTTP/1.1``. The third clause
  specifies a case-insensitive prefix match.

If all three clauses are true, then the ``disposition`` specifies a
synthetic response with status ``400 Bad Request`` -- an HTTP/1.1
request without a ``Host`` header is illegal.

The next element specifies that the request is processed in [pipe
mode](https://varnish-cache.org/docs/6.1/users-guide/vcl-built-in-subs.html#vcl-pipe)
for request method ``CONNECT``, or any non-standard request method:

```
    - conditions:
      - comparand: req.method
        compare: not-equal
        values:
          - GET
          - HEAD
          - PUT
          - POST
          - TRACE
          - OPTIONS
          - DELETE
          - PATCH
      disposition:
        action: pipe
```

Tnis configuration uses the ``not-equal`` comparison against an array
of strings, which evaluates to true if the ``equal`` comparison does
not evaluate to true against the array. It evaluates to true if the
request method (``req.method``) is equal to any string in the
array. If the ``not-equal`` comparison is true (hence the method is
either ``CONNECT`` or a method not specified by the HTTP standard),
then the ``disposition`` specifies ``pipe``. In pipe mode, Varnish
acts as a tunnel between the client and backend. This may be an
appropriate choice for a WebSockets client, for example.

The next element specifies that cache lookup is bypassed if the
request method is neither of ``GET`` or ``HEAD`` -- ``POST``
requests, for example, bypass cache lookup:

```
    - conditions:
      - comparand: req.method
        compare: not-equal
        values:
          - GET
          - HEAD
      disposition:
        action: pass
```

Setting the ``disposition`` to ``pass`` for requests whose responses
are known to be uncacheable can be advantageous, because potential
waiting due to [request
coalescing](https://varnish-cache.org/docs/6.1/users-guide/increasing-your-hitrate.html#passing-client-requests)
is avoided.

The next two elements are the ones that most commonly require an override
of the defaults: if a request has a ``Cookie`` or ``Authorization`` header,
then cache lookup is bypassed:

```
    - conditions:
      - comparand: req.http.Cookie
        compare: exists
      disposition:
        action: pass

    - conditions:
      - comparand: req.http.Authorization
        compare: exists
      disposition:
        action: pass
```

Both of the ``conditions`` use ``compare:exists``, which is true if
the header is present in the request. ``exists`` and ``not-exists``
may only be used when the ``comparand`` specifies a header.

To verify the configuration, as with other examples we use curl with
the ``-x`` (or ``--proxy``) option set to ``$IP:$PORT``, where ``$IP``
is the public address of the cluster, and ``$PORT`` is the port at
which it receives requests that are directed to the Ingress:

```
# The usual requests routed by the Ingress get the expected responses,
# as for built-in vcl_recv:
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: coffee-6c47b9cb9c-mrddp
[...]
URI: /coffee

$ curl -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-4vd7g
[...]
URI: /tea

$ curl -x $IP:$PORT -v http://cafe.example.com/beer
[...]
> GET http://cafe.example.com/beer HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 404 Not Found
[...]

# Requests using the PRI method get the response 405 Method Not
# Allowed.
$ curl -x $IP:$PORT -X PRI -v http://cafe.example.com/coffee
[...]
> PRI http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 405 Method Not Allowed
[...]

# To construct a request without a Host header using curl, we do
# not use the -x option, but set -H 'Host:' to remove the header.
# The response is 400 Bad Request as configured above.
curl -H 'Host:' -v http://$IP:$PORT/coffee
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee
> GET /coffee HTTP/1.1
> User-Agent: curl/7.52.1
> Accept: */*
> 
< HTTP/1.1 400 Bad Request
[...]

# To verify the pipe and pass dispositions, we view the Varnish log on
# one of the Pods on which Varnish is deployed, using kubetcl exec:
$ kubectl exec -it varnish-98498798b-qp5m6 -- varnishlog -n /var/run/varnish-home

# Requests with the CONNECT method are diverted to pipe mode:
$ curl -x $IP:$PORT -X CONNECT -v http://cafe.example.com/coffee
[...]
> CONNECT http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]

# In the Varnish log:
*   << Request  >> 32853     
[...]
-   ReqMethod      CONNECT
-   ReqURL         /coffee
[...]
-   VCL_call       RECV
-   VCL_return     pipe
[...]

# Requests whose method is neither GET nor HEAD are set to pass,
# bypassing cache lookup:
$ curl -x $IP:$PORT -X PUT -v http://cafe.example.com/coffee
[...]
> PUT http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]

*   << Request  >> 32852     
[...]
-   ReqMethod      PUT
-   ReqURL         /coffee
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]

# Requests with either of the Cookie or Authorization headers are set
# to pass.
$ curl -x $IP:$PORT -H 'Cookie: foo=bar' -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar
[...]

*   << Request  >> 32879     
[...]
-   ReqMethod      GET
-   ReqURL         /coffee
[...]
-   ReqHeader      Cookie: foo=bar
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]

```

## An alternative configuration

The next example demonstrates different choices about the disposition of
client requests. The main differences are:

* Cache lookups are permitted for requests that use cookies or basic
  authentication.

* Non-standard request methods are handled a bit differently.

Apply the configuration:

```
$ kubectl apply -f alt-builtin.yaml
```

Recall that if the ``req-disposition`` configuration is used at all,
everything in built-in ``vcl_recv`` is overridden; so the
configuration must include any features we wish to preserve. In
``alt-builtin.yaml``, two elements are included that are the same as
in ``builtin.yaml``, and bring about the same logic as in built-in
``vcl_recv``:

* Requests for the HTTP/1.1 protocol without a ``Host`` header get a
  synthetic 400 Bad Request response.

* Cache lookup is bypassed for requests whose method is neither of GET
  or HEAD.

The differences are:

* The stanzas in ``builtin.yaml`` that set the ``disposition`` to
  ``pass`` when ``Cookie`` or ``Authorization`` headers are present
  are left out.  Since processing proceeds to cache lookup if none of
  the ``conditions`` in ``req-disposition`` match the request, caching
  becomes possible for such requests.

* The only request method that invokes pipe mode is ``CONNECT``. This
  may be useful if you have client and backend applications that use a
  technique such as WebSockets, needing the "tunneling" feature of pipe
  mode.

* For all other non-standard request methods (including ``PRI``), a
  synthetic 405 Method Not Allowed response is generated.

This stanza invokes pipe mode for request method ``CONNECT``:

```
    - conditions:
      - comparand: req.method
        compare: equal
        values:
          - CONNECT
      disposition:
        action: pipe
```

The stanza for handling non-standard request methods is rewritten as:

```
    - conditions:
      - comparand: req.method
        compare: not-equal
        values:
          - GET
          - HEAD
          - PUT
          - POST
          - TRACE
          - OPTIONS
          - DELETE
          - PATCH
      disposition:
        action: synth
        status: 405
```

If your site does not have any application such WebSockets that
requires pipe mode, just leave out the stanza concerning ``CONNECT``,
and include ``CONNECT`` in the array of method names in the second
stanza.

You may use a configuration like this to limit the permitted request
methods more narrowly. For example, if none of your backend
applications support any method besides ``GET``, ``HEAD`` and
``POST``, then only include those method names in the ``values``
array.

Verification:

```
# Cache lookup is permitted for a request with a Cookie header.
# We verify this by checking the log.
$ curl -x $IP:$PORT -H 'Cookie: foo=bar' -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar
[...]

# VCL_return:hash in the log indicates that processing proceeds to
# cache lookup:
*   << Request  >> 33142     
[...]
-   ReqMethod      GET
-   ReqURL         /coffee
[...]
-   ReqHeader      Cookie: foo=bar
[...]
-   VCL_call       RECV
-   VCL_return     hash
[...]

# Requests with the CONNECT method go to pipe mode:
$ curl -x $IP:$PORT -X CONNECT -v http://cafe.example.com/coffee
[...]
> CONNECT http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]

*   << Request  >> 33171     
[...]
-   ReqMethod      CONNECT
-   ReqURL         /coffee
[...]
-   VCL_call       RECV
-   VCL_return     pipe
[...]

# Requests with any non-standard method get a 405 Method Not Allowed
# response.
$ curl -x $IP:$PORT -X HACK -v http://cafe.example.com/coffee
[...]
> HACK http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 405 Method Not Allowed
[...]
```

## Bypassing cache lookup for specific cookies

The next example illustrates a more fine-grained solution to permitting
cache lookups for requests with cookies. If the Cookie header contains
specific cookie names, with their values constrained to specific forms,
then cache lookup is bypassed, otherwise permitted. This may be used to
ensure that personalized responses are not cached, for example if the
request uses a cookie with a session or login token. But if the request
uses other cookies, then the response may be cacheable.

To apply the configuration:

```
$ kubectl apply -f pass-on-session-cookie.yaml
```

The configuration contains this stanza for request with cookies:

```
    - conditions:
      - comparand: req.http.Cookie
        compare: match
        values:
          - \bSESSIONID\s*=\s*[[:xdigit:]]{32}\b
          - \bLOGIN\s*=\s*\w+\b
      disposition:
        action: pass
```

The condition in this stanza uses ``compare:match``, indicating a
regex match with any of the patterns in ``values``, which are
interpreted as [RE2](https://github.com/google/re2/wiki/Syntax)
regular expressions.  The cookies in question are ``SESSIONID``, whose
value may be a 32 digit hex string; and ``LOGIN``, whose value may be
any string of word characters.  Cache lookup is bypassed if the Cookie
header's value matches either of the two patterns; otherwise cache
lookup is permitted for a request with cookies.

Remember that the ``req-disposition`` configuration also includes
other features we wish to preserve (such as status 400 for HTTP/1.1
requests with no Host header, or status 405 for non-standard request
methods).

Verification:
```
# The Varnish log shows pass for requests with cookies that match
# either of the two patterns:
$ curl -x $IP:$PORT -H 'Cookie: SESSIONID=0123456789abcdef0123456789abcdef' -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: SESSIONID=0123456789abcdef0123456789abcdef
[...]

*   << Request  >> 33011     
[...]
-   ReqMethod      GET
-   ReqURL         /coffee
[...]
-   ReqHeader      Cookie: SESSIONID=0123456789abcdef0123456789abcdef
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]

$ curl -x $IP:$PORT -H 'Cookie: LOGIN=foobar' -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: LOGIN=foobar
[...]

*   << Request  >> 252       
[...]
-   ReqMethod      GET
-   ReqURL         /coffee
[...]
-   ReqHeader      Cookie: LOGIN=foobar
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]

# Cache lookup proceeds if the Cookie header does not match either pattern:
$ curl -x $IP:$PORT -H 'Cookie: foo=bar' -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar
[...]

*   << Request  >> 269       
[...]
-   ReqMethod      GET
-   ReqURL         /coffee
[...]
-   ReqHeader      Cookie: foo=bar
[...]
-   VCL_call       RECV
-   VCL_return     hash
[...]
```

## Defining cacheability for URL path patterns

The next example shows a different kind of feature that may be
implemented with ``req-disposition`` -- defining requests as cacheable
or not cacheable based on URL path patterns.

Applying the configuration:

```
$ kubectl apply -f cacheability.yaml
```

The stanzas of interest in this configuration specify ``disposition``
as ``hash`` or ``pass`` based on whether the URL path matches sets of
patterns:

```
    - conditions:
      - comparand: req.url
        compare: match
        values:
          - \.png$
          - \.jpe?g$
          - \.css$
          - \.js$
      disposition:
        action: hash

    - conditions:
      - comparand: req.url
        compare: prefix
        values:
          - /interactive/
          - /basket/
          - /personal/
          - /dynamic/
      disposition:
        action: pass
```

The first stanza uses ``compare.match`` to specify matching the URL against
the regular expressions in ``values``, all of which describe file endings
(for typically cacheable content). If the URL matches any one of them, then
proceed to cache lookup (``action:hash``).

The second stanza uses ``compare:prefix``, to determine if the URL has a
prefix that is listed as one of the ``values``. If so, then cache lookup
is bypassed.

Verification:

```
# URLs that match the patterns for cacheability:
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/black.js
[...]
> GET http://cafe.example.com/coffee/black.js HTTP/1.1
> Host: cafe.example.com
[...]

# In the Varnish log:
*   << Request  >> 688       
[...]
-   ReqMethod      GET
-   ReqURL         /coffee/black.js
[...]
-   VCL_call       RECV
-   VCL_return     hash
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/sugar.css
[...]
> GET http://cafe.example.com/tea/sugar.css HTTP/1.1
> Host: cafe.example.com
[...]

*   << Request  >> 33416     
[...]
-   ReqMethod      GET
-   ReqURL         /tea/sugar.css
[...]
-   VCL_call       RECV
-   VCL_return     hash
[...]

# URLs with prefixes classified as non-cacheable
$ curl -x $IP:$PORT -v http://cafe.example.com/interactive/foo/bar
[...]
> GET http://cafe.example.com/interactive/foo/bar HTTP/1.1
> Host: cafe.example.com
[...]

# The log shows that we go to pass:
*   << Request  >> 719       
[...]
-   ReqMethod      GET
-   ReqURL         /interactive/foo/bar
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/dynamic/baz/quux
[...]
> GET http://cafe.example.com/dynamic/baz/quux HTTP/1.1
> Host: cafe.example.com
[...]

*   << Request  >> 33455     
[...]
-   ReqMethod      GET
-   ReqURL         /dynamic/baz/quux
[...]
-   VCL_call       RECV
-   VCL_return     pass
[...]
```

## Request white- and blacklisting

White- and blacklisting requests, based on properties of the client
request, are additional features made possible by ``req-disposition``,
demonstrated in the next example.

Applying the configuration:

```
$ kubectl apply -f url-whitelist.yaml
```

The URL whitelist is defined in this stanza:

```
    - conditions:
      - comparand: req.url
        compare: not-prefix
        values:
          - /tea/sugar/
          - /coffee/sugar/
      disposition:
        action: synth
        status: 403
```

This means that a synthetic 403 Forbidden response is sent for every
request whose URL path does not begin with one of the prefixes in
``values``.

The blacklist is defined with:

```
    - conditions:
      - comparand: req.url
        compare: prefix
        values:
          - /tea/sugar/black/
          - /coffee/sugar/black/
      disposition:
        action: synth
        status: 403
        reason: Blacklisted
```

In this case, the synthetic 403 response is generated for requests
whose URL path does begin with one of the prefixes in ``values``.
The ``reason`` setting sets the response line to "403 Blacklisted"
rather than the standard "403 Forbidden". In most cases, you can
leave out ``reason``, and Varnish sets the standard reason string
corresponding to the response code.

The combined effect is that requests are only permitted for URLs in
the whitelist, but not for URLs in the blacklist.

Of course your configuration can characterize the requests by other
means available in ``conditions``, for example by specifying regex
matching in ``compare``, and/or other properties of the request, such
as headers, in ``comparand``.

Verification:

```
# Requests matching the whitelist are permitted:
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/sugar/bar
[...]
> GET http://cafe.example.com/coffee/sugar/foo HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/sugar/bar
[...]
> GET http://cafe.example.com/tea/sugar/foo HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]

# Requests not matching the whitelist are forbidden:
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/baz
[...]
> GET http://cafe.example.com/coffee/baz HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 403 Forbidden
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/quux
[...]
> GET http://cafe.example.com/tea/quux HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 403 Forbidden
[...]

# Requests matching the blacklist are also forbidden. Notice that the
# "Blacklisted" reason string is used for these cases.
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/sugar/black/foo
[...]
> GET http://cafe.example.com/coffee/sugar/black/foo HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 403 Blacklisted
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/sugar/black/foo
[...]
> GET http://cafe.example.com/tea/sugar/black/foo HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 403 Blacklisted
[...]

```

## Defining a PURGE method

The final example demonstrates a way to make the Varnish
[purge](https://varnish-cache.org/docs/6.1/users-guide/purging.html)
facility available via a ``PURGE`` request method. When the processing
state is set to ``purge``, and the request is a cache hit, the cached
object and all of its variants are invlaidated, and Varnish send a
synthetic "200 Purged" response.

Applying the configuration:

```
$ kubectl apply -f purge-method.yaml
```

The ``PURGE`` method is defined with:

```
    - conditions:
      - comparand: req.method
        compare: equal
        values:
          - PURGE
      disposition:
        action: purge
```

This simply diverts to ``action:purge`` whenever the request method is
``PURGE``. You will almost certainly want to also define authorization
for use of purging via request, for example with an
[ACL](/examples/acl) or [basic
authentication](/examples/authentication), so that only trusted users
are able to purge cache entries.

Verification:

```
$ curl -X PURGE -x $IP:$PORT -v http://cafe.example.com/coffee
[...]
> PURGE http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 Purged
[...]

```
