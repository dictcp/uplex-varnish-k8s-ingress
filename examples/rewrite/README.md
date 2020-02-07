# Sample rewrite rules for headers and URL paths

The sample manifest in this folder configures rewrite operations for
HTTP headers and URL paths, to be executed by a Varnish Service
implementing Ingress. The specifications take the form of objects in
the ``rewrites`` array of a VarnishConfig resource.

See the [docs](/docs/ref-varnish-cfg.md) for the technical reference
for specifying rewrites.

The examples apply to the Ingress and Services defined in the
["cafe" example](/examples/hello).

Apply the rewrite configuration with:

```
$ kubectl apply -f rewrite.yaml
```

The rewrites can then be verified with the sample commands shown in
the following.

## Rewriting URL prefixes

The first two examples in the sample configuration demonstrate different
ways to rewrite the prefix of a URL path. Since the
[Ingress](/examples/hello/cafe-ingress.yaml) defined in the
["cafe" example](/examples/hello) specifies routing rules based on the
URL prefix -- ``/coffee/*`` requests are forwarded to the coffee-svc,
and ``/tea/*`` requests to the tea-svc -- this is a way to allow
requests with additional URL prefixes to be forwarded to the two Services.

The first example defines alternative prefixes for the coffee-svc:

```
    - target: req.url
      compare: match
      method: sub
      rules:
        - value: /espresso(/|$)
          rewrite: /coffee\1
        - value: /capuccino(/|$)
          rewrite: /coffee\1
        - value: /latte(/|$)
          rewrite: /coffee\1
        - value: /macchiato(/|$)
          rewrite: /coffee\1
        - value: /ristretto(/|$)
          rewrite: /coffee\1
        - value: /americano(/|$)
          rewrite: /coffee\1
      match-flags:
        anchor: start
```

The overall purpose is to rewrite URLs with matching prefixes so that
they begin with ``/coffee``, and hence match the Ingress rule.

The ``target`` is ``req.url``, or the client request URL path. Since
``source`` is left out, it is implicitly the same as the ``target``,
so this configuration specifies an in-place rewrite of the URL.

Since the ``target`` and ``source`` are ``req.url``, the rewrite is
executed in the VCL subroutine ``vcl_recv``; that is, just after the
client request header (including the URL path) is received.

The ``compare:match`` setting means that regular expression matching
is executed for this rewrite; since ``match`` is the default value,
the ``compare`` field could have been left out. When ``compare:match``
is specfied, the ``source`` object (in this case ``req.url``) is
matched against the regexen that appear in the ``value`` fields of the
``rules`` array. For rewrites, regexen have the syntax and semantics
of [RE2](https://github.com/google/re2/wiki/Syntax).

The ``anchor:start`` match flag means that the regexen are anchored at
start-of-string, as if each of them begins with a ``^``. This
convenience ensures that anchoring is maintained if the patterns are
changed; you can't make the mistake of adding a new pattern and
forgetting the ``^``.

The ``select`` field is left out, and hence has the default value
``unique``, meaning that if any URL matches one of the patterns, it
must match exactly one of them (which in fact is the only possibility
with these patterns).

The ``method:sub`` setting means that, if the URL matches one of the
patterns, then the matching portion is substituted with the string in
the ``rewrite`` field that corresponds to the match. In this example,
the rewrite strings happen to be all the same, but of course they
could each be different. With the ``sub`` method (as also with
``suball`` and ``rewrite``), backreferences such as ``\1`` are
replaced with the corresponding capturing subgroup from the matching
pattern.

In the example, the end of the matching prefix must be either ``/`` or
end-of-string (``$``). This means that, for example, the URL
``/espresso`` (without the trailing slash) is rewritten as ``/coffee``
and ``/espresso/doppio`` as ``/coffee/doppio``; but a URL such as
``/espresso-grande`` does not match any pattern, and is not rewritten.

To verify the rewrite, we send requests with the alternative URL
prefixes, and check the response to ensure that it came from the
coffee-svc Service.  As with other examples, we use curl with the
``-x`` (or ``--proxy``) option set to ``$IP:$PORT``, where ``$IP`` is
the public address of the cluster, and ``$PORT`` is the port at which
it receives requests that are directed to the Ingress:

```
# URL path /espresso (without the trailing slash) is rewritten as /coffee.
$ curl -x $IP:$PORT -v http://cafe.example.com/espresso
[...]
> GET http://cafe.example.com/espresso HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: coffee-6c47b9cb9c-q7xrl
[...]
URI: /coffee
[...]

# /capuccino/ (with the trailing slash) is rewritten as /coffee/.
$ curl -x $IP:$PORT -v http://cafe.example.com/capuccino/
[...]
> GET http://cafe.example.com/capuccino/ HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: coffee-6c47b9cb9c-q7xrl
[...]
URI: /coffee/
[...]

# /latte/grande is rewritten as /coffee/grande.
$ curl -x $IP:$PORT -v http://cafe.example.com/latte/grande/
[...]
> GET http://cafe.example.com/latte/grande/ HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: coffee-6c47b9cb9c-q7xrl
[...]
URI: /coffee/grande/
[...]
```

The next example performs a similar function to rewrite URL prefixes
for the tea-svc, but there are some differences from the previous
example worth considering in detail:

```
    - target: req.url
      compare: prefix
      rules:
        - value: /camomille
          rewrite: /tea
        - value: /earl-grey
          rewrite: /tea
        - value: /chai
          rewrite: /tea
        - value: /green
          rewrite: /tea
        - value: /hibiscus
          rewrite: /tea
        - value: /oolong
          rewrite: /tea
      method: sub
```

As with the previous example, the ``target`` and ``source`` is
``req.url``, so this is another in-place rewrite of the URL. The
``method`` is ``sub``, and this configuration specifies replacing the
matching portion of the URL with the string in the corresponding
``rewrite`` field.

``compare`` in this case is ``prefix``, meaning that the match is
against a fixed string prefix. That is, a URL matches if it has a
prefix that is one of the strings specified as a ``value`` in the
``rules``. With ``compare:prefix``, the strings in the ``value``
fields are not patterns; any regular expression metacharacter that may
appear in a ``value`` is matched literally, and has no special
meaning.

Verification with curl:

```
# /camomille (without the trailing slash) is rewritten as /tea.
$ curl -x $IP:$PORT -v http://cafe.example.com/camomille
[...]
> GET http://cafe.example.com/camomille HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-6vcz9
[...]
URI: /tea
[...]

# /earl-grey/ (with the trailing slash) is rewritten as /tea/.
$ curl -x $IP:$PORT -v http://cafe.example.com/earl-grey/
[...]
> GET http://cafe.example.com/earl-grey/ HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-9vl28
[...]
URI: /tea/
[...]

# /chai/link is rewritten as /tea/link.
$ curl -x $IP:$PORT -v http://cafe.example.com/chai/link
[...]
> GET http://cafe.example.com/chai/link HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-5fgwr
[...]
URI: /tea/link
[...]

# /chain/link is rewritten as /tean/link, which may not be desired.
$ curl -x $IP:$PORT -v http://cafe.example.com/chain/link
[...]
> GET http://cafe.example.com/chain/link HTTP/1.1
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-5fgwr
[...]
URI: /tean/link
[...]
```

The second configuration, which uses ``compare:prefix`` for
alternative ``/tea`` prefixes, is easier to read and write than the
first example with regular expressions. Depending on the complexity of
the match and the strings to be matched, prefix matches may run faster
than regex matches (but that depends on many factors).

But the last curl example shows that the ``prefix`` match may not
fulfill all requirements of the use case. The ``prefix`` match can
handle the cases with and without the trailing slash after the URL
prefix, but it cannot prevent a URL ``/chain/link`` from being
rewritten as ``/tean/link``.

Another way to use the ``prefix`` match to avoid that result would be
to specify the trailing slash in the ``value`` fields. In that case,
the rewrite cannot handle the case with no trailing slash, but if your
site does not use any such URLs, then the rewrite would be suitable.

## Fixed string equality and rewrites

The next example in the sample manifest rewrites the ``Host`` header
for certain values, so that the request can be handled by the Ingress
(which requires the Host ``cafe.example.com``):

```
    - target: req.http.Host
      compare: equal
      rules:
        - value: my-cafe.com
          rewrite: cafe.example.com
        - value: my-example.com
          rewrite: cafe.example.com
        - value: ingress.example.com
          rewrite: cafe.example.com
        - value: varnish.example.com
          rewrite: cafe.example.com
        - value: atomic-cafe.com
          rewrite: cafe.example.com
      method: replace
```

As with the previous examples, the ``source`` is implicitly the same
as the ``target``, so this is an in-place rewrite of
``req.http.Host``, or the client request header ``Host``.

``compare:equal`` means that the Host header is matched for fixed
string equality with the strings in the ``value`` fields of the
``rules``. As with the ``prefix`` match, all characters in a ``value``
are matched literally, with no special meanings. There is no need to
specify an ``anchor``; the string matches from start to end, or not at
all.

The effect is that any of the given strings for the Host header is
rewritten to ``cafe.example.com``, which happens to be the only Host
specified in the example Ingress. We verify with curl by checking that
requests with one of these Host values is forwarded to a Service, but
for other values of Host we get the 404 response:

```
# For Host values in the list, we get the coffee-svc or tea-svc.
$ curl -x $IP:$PORT -v http://my-cafe.com/coffee
[...]
> GET http://my-cafe.com/coffee HTTP/1.1
> Host: my-cafe.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: coffee-6c47b9cb9c-q7xrl
[...]

$ curl -x $IP:$PORT -v http://my-example.com/tea
[...]
> GET http://my-example.com/tea HTTP/1.1
> Host: my-example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
Server name: tea-58d4697745-6vcz9
[...]

# Unknown Host values get the 404 response.
$ curl -x $IP:$PORT -v http://ricks.cafe.americain.com/coffee
[...]
> GET http://ricks.cafe.americain.com/coffee HTTP/1.1
> Host: ricks.cafe.americain.com
[...]
> 
< HTTP/1.1 404 Not Found
[...]
```

## Extracting a Cookie value into a header

The next example demonstrates a way to extract the value of a cookie
into another header:

```
    - target: resp.http.Session-Token
      source: req.http.Cookie
      rules:
        - value: \bmysession\s*=\s*([^,;[:space:]]+)
          rewrite: \1
      method: rewrite
```

In this case, the ``source`` differs from the ``target``. Matches are
executed against the source object ``req.http.Cookie``, the client request
header Cookie. If the match succeeds, a string is extracted from
Cookie and written to the target object ``resp.http.Session-Token``, the
client response header ``Session-Token``.

The ``compare`` field has the default value ``match``, and the
``method`` is ``rewrite``, meaning that the string in the ``rewrite``
field is written to the target, using the backreference extracted from
the pattern in ``value``, and ignoring non-matched portions of the
``source``.

The capturing pattern that forms backref 1 expresses the cookie value
as a string of any characters that are none of comma, semicolon or
whitespace. If there are narrower restrictions on the lexical form of
your cookie values (say, all hex digits, or characters from a base64
encoding), you can use a narrower definition in the pattern.

Verification with curl:

```
$ curl -x $IP:$PORT -v -H 'Cookie: foo=bar; mysession=4711; baz=quux' http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar; mysession=4711; baz=quux
> 
< HTTP/1.1 200 OK
[...]
< Session-Token: 4711
[...]

$ curl -x $IP:$PORT -v -H 'Cookie: foo=bar; mysession=bazquux' http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar; mysession=bazquux
> 
< HTTP/1.1 200 OK
[...]
< Session-Token: bazquux
[...]
```

## Setting the cache disposition in the X-Cache header

The next sequence of rewrites implements a common use case for
Varnish: set the header X-Cache in both the client request and
response to one of the values "HIT", "MISS" or "PASS", to expose the
disposition of the request with respect to the cache. These examples
make use of the ``vcl-sub`` field to control execution of the rewrites
in specific VCL subroutines:

```
    - target: req.http.X-Cache
      vcl-sub: hit
      rules:
        - rewrite: HIT
      method: replace

    - target: req.http.X-Cache
      vcl-sub: miss
      rules:
        - rewrite: MISS
      method: replace

    - target: req.http.X-Cache
      vcl-sub: pass
      rules:
        - rewrite: PASS
      method: replace

    - target: resp.http.X-Cache
      source: req.http.X-Cache
      method: replace
```

In the first three of these, the ``vcl-sub`` field specifies execution
of the rewrite in the subroutines ``vcl_hit``, ``vcl_miss`` or
``vcl_pass``, so as to set the request header to the proper
value. Each of them specifies ``method:replace``, with exactly one
element of the ``rules`` array, and that element has only the
``rewrite`` field. That means that the string in ``rewrite`` is
written unconditionally to the ``target`` object (the client request
header).

The fourth rewrite specifies ``method:replace`` with no rules, which
means that the value of the ``source`` is unconditionally copied to
the ``target``. This has the effect of copying the value from the
request header to the response header. (If you don't want to expose
X-Cache in responses, then leave out the fourth rewrite stanza).

The rewrite can be verified with curl by checking the response header:

```
$ curl -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< X-Cache: MISS
[...]

# By the default caching rules, a request is set to pass if there is
# a Cookie header.
$ curl -H 'Cookie: foo=bar' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: foo=bar
> 
< HTTP/1.1 200 OK
[...]
< X-Cache: PASS
[...]
```

## Rewriting the backend request URL

The next example is another in-place rewrite of a URL, in this case
the URL path in the backend request (``bereq.url``):

```
    - target: bereq.url
      rules:
        - value: ^/coffee/([^/]+)/([^/]+)(.*)
          rewrite: /coffee/\2/\1\3
      method: rewrite
```

The effect is that, if the URL begins with ``/coffee/`` and has second
and third backslash-separated components, then those components are
swapped in the backend request.

Since the ``vcl-sub`` field is not specified, the rewrite is executed
in the VCL subroutine ``vcl_backend_fetch`` (just before the backend
request is sent), since the ``target`` and ``source`` are both
``bereq.url``. So the rewrite is executed in backend context, after
Ingress rules are evaluated, which happens in client context based on
the incoming request.  Remember that if there is a cache hit, then
there is no backend request.

This rewrite can be verified with curl by checking the URI reflected
back in the response body, which shows the path as received by the
backend application:

```
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/foo/bar
[...]
> GET http://cafe.example.com/coffee/foo/bar HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
URI: /coffee/bar/foo
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/coffee/baz/quux
[...]
> GET http://cafe.example.com/coffee/baz/quux HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< 
[...]
URI: /coffee/quux/baz
[...]
```

## Deleting headers

The next example uses ``method:delete`` to specify a rewrite that
unconditionally deletes ``resp.http.Server``, the client response
header Server:

```
    - target: resp.http.Server
      method: delete
```

The rewrite can be verified with a client like curl by observing that
the Server response header, which is ordinarily forwarded from the
backend application, is never present in the response from Varnish.

The following rewrite specifies that the Via client response header is
deleted if the client request header Delete-Via has one of the given
values:

```
    - target: resp.http.Via
      method: delete
      source: req.http.Delete-Via
      compare: equal
      rules:
        - value: "true"
        - value: "yes"
        - value: "on"
        - value: "1"
      match-flags:
        case-sensitive: false
```

In this case, the ``source`` object is matched for case-insensitive
equality with the strings that appear in the ``value`` fields. None of
the objects in the ``rules`` array has a ``rewrite`` field, since
rewrite strings are not relevant for deletion.

Verification:

```
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Via: 1.1 varnish (Varnish/6.3)
[...]

# With an appropriate value for the Delete-Via request header, the Via
# header does not appear in the response.
$ curl -H 'Delete-Via: TRUE' -x $IP:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> Delete-Via: TRUE
> 
< HTTP/1.1 200 OK
[...]
```

``method:delete`` may only be used with a header as the ``target``,
never with ``req.url`` or ``bereq.url`` -- you can't delete the URL.

## Rewriting a header from another header, and from a fixed string

This example shows the general form for copying one header to another:
specify the headers with ``source`` and ``target``, and set ``method``
to ``replace``:

```
    - target: resp.http.Replace-Hdr-Target
      source: req.http.Replace-Hdr-Src
      method: replace
```

This unconditionally copies the value of the client request header
Replace-Hdr-Src to the client response header Replace-Hdr-Target:

```
$ curl -x $IP:$PORT -v -H 'Replace-Hdr-Src: the replacements' http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Replace-Hdr-Src: the replacements
> 
< HTTP/1.1 200 OK
[...]
< Replace-Hdr-Target: the replacements
[...]
```

To unconditionally set the value of a header to a fixed string, use
``method:replace`` and one element in the ``rules`` array. The one
rule has the ``rewrite`` field set to the string, and no ``value``
field:

```
    - target: resp.http.Replace-String-Target
      rules:
        - rewrite: ReplaceString
      method: replace
```

This sets the client response header Replace-String-Target to the
string "ReplaceString":

```
$ curl -x $IP:$PORT -v http://cafe.example.com/coffee
[...]
> GET http://cafe.example.com/coffee HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Replace-String-Target: ReplaceString
[...]
```

## Appending and prepending strings

The next group of examples illustrates the use of ``method:append``
and ``method:prepend`` to concatenate strings. In this example, the
fixed string "AppendString" is unconditionally appended to the value
of the ``source`` (the client request header Append-String-Src), and
the result is written to the ``target`` (the client response header
Append-String-Target):

```
    - target: resp.http.Append-String-Target
      source: req.http.Append-String-Src
      rules:
        - rewrite: AppendString
      method: append
```

Since the rule has no ``value``, the string is appended
unconditionally, even if there is no request header
Append-String-Src. The result in that case is that just the string
("AppendString") is written to the ``target`` (the response header):

```
# Append the string from the rewrite specification to the request header.
$ curl -H 'Append-String-Src: foobar' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Append-String-Src: foobar
> 
< HTTP/1.1 200 OK
[...]
< Append-String-Target: foobarAppendString
[...]

# If the request header does not exist, just write the string to the response
# header.
$ curl -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Append-String-Target: AppendString
[...]
```

To append the string conditionally (for example, only if the request
header exists), set a value for the ``value`` field; then the append
is only executed if the ``source`` passes a comparison with the
``value``. When ``compare`` has the default value ``match`` for a
regex match, and ``value`` is set to the pattern ``.`` for "match any
character", the append is only executed if the request header exists
and is non-empty:

```
    - target: resp.http.Append-Rule-Target
      source: req.http.Append-Rule-Src
      rules:
        - value: .
          rewrite: AppendString
      method: append
```

Verification:

```
$ curl -H 'Append-Rule-Src: bazquux' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Append-Rule-Src: bazquux
> 
< HTTP/1.1 200 OK
[...]
< Append-Rule-Target: bazquuxAppendString
[...]
```

To append the value of a header ``source`` to another header
``target``, specify the ``source`` and ``target`` with no rules:

```
    - target: req.http.Append-Hdr-Target
      source: req.http.Append-Hdr-Src
      method: append
```

This example has only request headers as the source and target, so we need
to verify its effect by reading the Varnish log (since rewritten request
headers cannot be seen in the curl response):

```
# Send the curl request
$ curl -H 'Append-Hdr-Target: foo' -H 'Append-Hdr-Src: bar' -x $IP:$PORT -v http://cafe.example.com/tea

# Check the result with varnishlog
*   << Request  >> 101540    
-   Begin          req 101539 rxreq
[...]
-   ReqMethod      GET
-   ReqURL         /tea
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      Append-Hdr-Target: foo
-   ReqHeader      Append-Hdr-Src: bar
-   ReqHeader      Host: cafe.example.com
[...]
-   ReqUnset       Append-Hdr-Target: foo
-   ReqHeader      Append-Hdr-Target: foobar
[...]
```

With ``method:prepend``, the order of concatenation is reversed -- a
string is prepended before the value of the target. The means for
specifying the string to prepend are the same as shown above for
append.

To prepend a fixed string -- here "PrependString" is unconditionally
prepended to the request header Prepend-String-Src:

```
    - target: resp.http.Prepend-String-Target
      source: req.http.Prepend-String-Src
      rules:
        - rewrite: PrependString
      method: prepend
```

To conditionally prepend the string (only if Prepend-Rule-Src exists
and is non-empty):

```
    - target: resp.http.Prepend-String-Target
      source: req.http.Prepend-String-Src
      rules:
        - rewrite: PrependString
      method: prepend
```

To prepend a header to another header:

```
    - target: req.http.Prepend-Hdr-Target
      source: req.http.Prepend-Hdr-Src
      method: prepend
```

Verification:

```
# Prepend "PrependString" to the value of the request header
# Prepend-String-Src, and write the result to the response header
# Prepend-String-Target:
$ curl -H 'Prepend-String-Src: foobar' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Prepend-String-Src: foobar
> 
< HTTP/1.1 200 OK
[...]
< Prepend-String-Target: PrependStringfoobar
[...]

# If the request header Prepend-Rule-Src exists, prepend "PrependString" to
# its value, and write the result to the response header Prepend-Rule-Target:
$ curl -H 'Prepend-Rule-Src: bazquux' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Prepend-Rule-Src: bazquux
> 
< HTTP/1.1 200 OK
[...]
< Prepend-Rule-Target: PrependStringbazquux
[...]

# Prepend the value of the request header Prepend-Hdr-Src to the request
# header Prepend-Hdr-Target:
$ curl -H 'Prepend-Hdr-Target: foo' -H 'Prepend-Hdr-Src: bar' -x $IP:$PORT -v http://cafe.example.com/tea

# Check the result in the Varnish log:
*   << Request  >> 5460      
-   Begin          req 5459 rxreq
[...]
-   ReqMethod      GET
-   ReqURL         /tea
-   ReqProtocol    HTTP/1.1
[...]
-   ReqHeader      Prepend-Hdr-Target: foo
-   ReqHeader      Prepend-Hdr-Src: bar
[...]
-   ReqUnset       Prepend-Hdr-Target: foo
-   ReqHeader      Prepend-Hdr-Target: barfoo
[...]
```

## Select a rewrite from multiple matching rules

If the ``compare`` field is set to ``equal`` to specify string
equality matches, then if any string in a ``value`` field in the
``rules`` matches, it always matches exactly one of them (since the
same value for ``value`` may not appear more than once in a ``rules``
array).  So the ``rewrite`` is always uniquely selected when an
``equal`` comparison succeeds.

But for the comparisons specified by ``compare:match`` (regex match)
or ``compare:prefix`` (fixed prefix match), it is possible that more
than one ``value`` matches, depending on the patterns used for the
values. For example, for either of regex or prefix matches, the string
``/tea/foo/bar/baz/quux`` matches all four values in the following
``rules``:

```
      rules:
        - value: /tea/foo/bar/baz/quux
          rewrite: Quux
        - value: /tea/foo/bar/baz
          rewrite: Baz
        - value: /tea/foo/bar
          rewrite: Bar
        - value: /tea/foo
          rewrite: Foo
```

The ``select`` field is used to specify the rule to be applied in such
a situation. The default value of ``select`` is ``unique``, which
means that the rule identified by a comparison must be uniquely
determined, or else the rewrite fails. In all of the examples
considered above, ``select`` has defaulted to ``unique``, and in fact
only unique matches have been possible in those examples (because the
rules did not overlap in this way).

Other possible values for ``select`` depend on whether ``compare`` is
set to ``match`` or ``prefix``:

* ``unique`` (default for both ``compare:match`` and
  ``compare:prefix``): select the unique matching rule. The rewrite
  fails if the match is not unique.

* ``first`` (permitted for both ``match`` and ``prefix``): select the
  first matching rule, in the order given by the ``rules`` array

* ``last`` (permitted for both ``match`` and ``prefix``): select the
  last matching rule in the ``rules`` array

* ``exact`` (only ``prefix``): select the rule for which the ``value``
  is exactly equal to the string to be matched. The rewrite fails if
  there is no exact match.

    * For example, if the ``rules`` include ``/foo/`` and ``/foo/bar``
      for a prefix match, and the string to be matched is exactly
      ``/foo/bar``, select the rule corresponding to ``/foo/bar``.

* ``shortest`` (only ``prefix``): select the rule with the shortest
  matching prefix

* ``longest`` (only ``prefix``): select the rule with the longest
  matching prefix

"Failure" in the case of ``select:unique`` and ``select:exact`` means
that VCL failure is invoked if there is more than one matching rule
for the comparison. In most cases, this means that a response with
status 503 and the reason ``VCL failed`` is returned for the
request. This is a "fail fast" measure taken to ensure that the unique
or exact match is satisfied -- test your data and rules to make sure
that the requirement is satisfied, to avoid the failures.

The first example in this group uses ``select:first`` to pick the
rewrite rule from the rules shown above:

```
    - target: resp.http.Select-First
      source: req.url
      rules:
        - value: /tea/foo/bar/baz/quux
          rewrite: Quux
        - value: /tea/foo/bar/baz
          rewrite: Baz
        - value: /tea/foo/bar
          rewrite: Bar
        - value: /tea/foo
          rewrite: Foo
      compare: prefix
      method: replace
      select: first
```

This tests the client request URL for a prefix that is specified by one
of the rules, and if one is found, select the first one that matches.
The string in the ``rewrite`` field for the corresponding rule is then
written to the client response header Select-First:

```
$ curl -x $IP:$PORT -v http://cafe.example.com/tea/foo/bar/baz/quux/4711
[...]
> GET http://cafe.example.com/tea/foo/bar/baz/quux/4711 HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Select-First: Quux
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/foo/bar
[...]
> GET http://cafe.example.com/tea/foo/bar HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Select-First: Bar
[...]
```

The next example is very similar to the previous one, except that it
uses ``select:longest`` to specify the rewrite for the longest
matching prefix.  This has the same effects of the previous example,
but does not depend on the order of elements of the ``rules`` array:

```
    - target: resp.http.Select-Longest
      source: req.url
      rules:
        - value: /tea/foo
          rewrite: Foo
        - value: /tea/foo/bar/baz
          rewrite: Baz
        - value: /tea/foo/bar
          rewrite: Bar
        - value: /tea/foo/bar/baz/quux
          rewrite: Quux
      compare: prefix
      method: replace
      select: longest
```

```
$ curl -x $IP:$PORT -v http://cafe.example.com/tea/foo/4711/0815
[...]
> GET http://cafe.example.com/tea/foo/4711/0815 HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Select-Longest: Foo
[...]

$ curl -x $IP:$PORT -v http://cafe.example.com/tea/foo/bar/a/b/
[...]
> GET http://cafe.example.com/tea/foo/bar/a/b/ HTTP/1.1
> Host: cafe.example.com
[...]
> 
< HTTP/1.1 200 OK
[...]
< Select-Longest: Bar
[...]
```

In the final example, consider a number of cookies that may appear in
the cookie header in any order. The requirement is to extract the
value from one of them, and write the cookie name and value as a
key:value pair to a header (the client response header Cookie-Select
in the example).

If an order of preference can be specified for the cookies of
interest, then the ``rules`` array can be used to order the matching
rules, and ``select`` can be used to pick the preferred value. In this
case, we pick the last matching rule in the array:

```
    - target: resp.http.Cookie-Select
      source: req.http.Cookie
      rules:
        - value: \bcookie1\s*=\s*([^,;[:space:]]+)
          rewrite: cookie1:\1
        - value: \bcookie2\s*=\s*([^,;[:space:]]+)
          rewrite: cookie2:\1
        - value: \bcookie3\s*=\s*([^,;[:space:]]+)
          rewrite: cookie3:\1
        - value: \bcookie4\s*=\s*([^,;[:space:]]+)
          rewrite: cookie4:\1
        - value: \bcookie5\s*=\s*([^,;[:space:]]+)
          rewrite: cookie5:\1
      method: rewrite
      select: last
```

Verification with curl shows that the last matching rule is chosen for
the rewrite, regardless of the order of cookies in the Cookie header:

```
$ curl -H 'Cookie: cookie2=val2; cookie3=val3; cookie1=val1' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: cookie2=val2; cookie3=val3; cookie1=val1
> 
< HTTP/1.1 200 OK
[...]
< Cookie-Select: cookie3:val3
[...]

$ curl -H 'Cookie: cookie3=val3; cookie4=val4' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: cookie3=val3; cookie4=val4
> 
< HTTP/1.1 200 OK
[...]
< Cookie-Select: cookie4:val4
[...]

$ curl -H 'Cookie: cookie5=val5; cookie4=val4; cookie3=val3' -x $IP:$PORT -v http://cafe.example.com/tea
[...]
> GET http://cafe.example.com/tea HTTP/1.1
> Host: cafe.example.com
[...]
> Cookie: cookie5=val5; cookie4=val4; cookie3=val3
> 
< HTTP/1.1 200 OK
[...]
< Cookie-Select: cookie5:val5
[...]
```

Effective use of ``select`` involves knowledge of the patterns to be
matched, the data against which they are matched, and possibly an
ordering of preferences as reflected in the order of the ``rules``
array. With appropriate choices, some sophisticated use cases can be
solved efficiently with a relatively simple configuration.
