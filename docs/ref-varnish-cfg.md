# VarnishConfig Custom Resource reference

This is the authoritative reference for the ``VarnishConfig``
[Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
which is defined in this project to specify special configurations and
features for Varnish clusters running as Ingress, in addition to the
routing rules given in an Ingress specification.

If you only intend to use Varnish-as-Ingress Services to implement the
routing rules for an Ingress, then no ``VarnishConfig`` resource needs to
be defined. Use the ``VarnishConfig`` resource to apply additional
configurations such as [self-sharding](/docs/self-sharding.md).

Constraints on individual properties in ``VarnishConfig`` are checked
against validation rules when the manifest is applied, so you may get
immediate feedback about invalid values from a ``create`` or ``apply``
command for ``kubectl``. Other constraints, such as legal relations
between values or valid VCL syntax, cannot currently be checked until
the controller attempts to load the definition, and hence will not be
reported at apply time. Check the log of the controller and Events
created by the controller for error conditions -- these may include
error messages from the VCL compiler.

Examples for the use of ``VarnishConfig`` resources can be found in
the [``examples/``](/examples) folder.

## Custom Resouce definition

The Custom Resource is created with the ``CustomResourceDefinition`` defined
in [``varnishcfg-crd.yaml``](/deploy/varnishcfg-crd.yaml) in the
[``deploy/``](/deploy) folder:

```
$ kubectl apply -f deploy/varnishcfg-crd.yaml
```

## API Group, version and resource names

The API group in use for this project is
``ingress.varnish-cache.org``, currently at version ``v1alpha1``. So a
manifest specifying a ``VarnishConfig`` resource MUST begin with:
```
apiVersion: "ingress.varnish-cache.org/v1alpha1"
kind: VarnishConfig
```

You can choose any ``name`` and ``namespace`` in the ``metadata``
section.  ``VarnishConfig`` has Namespaced scope, so its name must be
unique in a namespace, and its content is applied to Varnish Services
in the same namespace.

Existing ``VarnishConfig`` resources can be referred to in ``kubectl``
commands as ``varnishconfig``, ``varnishconfigs`` or with the short
name ``vcfg``:

```
$ kubectl get varnishconfigs -n my-namespace
$ kubectl describe vcfg my-vcfg
```

## ``spec``

The ``spec`` section of a ``VarnishConfig`` is required.

### ``spec.services``

The ``spec.services`` array is required, and MUST have at least one
element:

```
spec:
  # The services array is required and must have at least one element.
  # Lists the Service names of Varnish services in the same namespace
  # to which this config is to be applied.
  services:
    - my-ingress
```

The strings in the ``services`` array MUST match the Service names of
Varnish Services to implement Ingress in the same namespace as the
``VarnishConfig`` Resource. The configuration in the Resource is
applied to those resources -- this makes it possible to have more than
one Varnish-as-Ingress Service in a namespace with different
configurations.

### ``spec.self-sharding``

The ``self-sharding`` object is optional. If it is present in the
manifest, then the [self-sharding](/docs/self-sharding.md) feature is
implemented for Services listed in the ``services`` array.

All of the properties of a ``self-sharding`` object are optional, and
default values hold for any properties that are not specified. To
specify self-sharding with all default values, just use an empty object:

```
spec:
  # Implement self-sharding with defaults for all properties:
  self-sharding: {}
```

Properties that may be specifed for ``self-sharding`` are:

* ``max-secondary-ttl``: string
* ``probe``: object

If specified, ``max-secondary-ttl`` MUST have the form of the VCL
[DURATION type](https://varnish-cache.org/docs/6.1/reference/vcl.html#durations)
(examples are ``90s`` for ninety seconds, or ``2m`` for two
minutes). This value is the TTL for "secondary" caching -- the upper
bound for a cached response forwarded from the "primary" Varnish instance
for a cacheable response (see the
[self-sharding document](/docs/self-sharding.md) for details).
``max-secondary-ttl`` defaults to ``5m`` (5 minutes).

The ``probe`` object specifies the health probes that Varnish instances
in a cluster use for one another (since they are defined as backends
for one another). Its properties are:

* ``timeout``: string
* ``interval``: string
* ``initial``: integer
* ``window``: integer
* ``threshold``: integer

These properties configure the corresponding values for
[health probes](https://varnish-cache.org/docs/6.1/reference/vcl.html#probes),
and they default to the default values for Varnish probes. If the
``probe`` object is left out altogether, then defaults hold for all of
its properties.

``timeout`` and ``interval`` MUST have the form of VCL DURATIONs, and
each of ``initial``, ``window`` and ``threshold`` MUST be >= 0.
``window`` and ``threshold`` MUST also be <= 64, and ``threshold``
MAY NOT be larger than ``window``.

Validation for ``VarnishConfig`` will report errors in the individual
fields at apply time, for example if the VCL DURATION properties do
not have the proper form. The ``threshold`` <= ``window`` constraint
is checked at VCL load time; if violated, it is reported in the
controller log and in Events generated by the controller for the
``VarnishConfig`` resource (with the error message from the VCL
compiler).

Example:
```
spec:
  self-sharding:
    # Any of these properties may be left out, in which case default
    # values hold.
    max-secondary-ttl: 2m
    probe:
      timeout: 6s
      interval: 6s
      initial: 2
      window: 4
      threshold: 3
```

### ``spec.auth``

The ``auth`` object is optional, and if present it contains a
non-empty array of specifications for authentication protocols (Basic
or Proxy) to be implemented by Varnish Services listed in the
``services`` array. See [RFC7235](https://tools.ietf.org/html/rfc7235)
for the HTTP Authentication standard.

For each element of ``auth``, these two fields are required:

* ``realm``: string identifying the realm or "protection space" for
  authentication
* ``secretName``: the name of a Secret in the same namespace as the
  VarnishConfig resource and Varnish Services that contains the
  username/password credentials for authentication

The Secret identified by ``secretName`` MUST have the label
``app: varnish-ingress``; otherwise it is ignored by the Ingress
controller, and the authentication scheme will not be implemented.

The key-value pairs in the Secret are the username-password pairs to
be used for authentication.

These fields in the elements of ``auth`` are optional:

* ``type`` (string): one of the values ``basic`` or ``proxy`` to
  specify the authentication protocol, ``basic`` by default
* ``utf8`` (boolean): if ``true``, then the ``charset="UTF-8"``
  field is added to the ``*-Authenticate`` response header
  (``WWW-Authentcate`` or ``Proxy-Authenticate``) in the case of
  authentication failures, to advise clients that UTF-8 character
  encoding is used for the username/password (see
  [RFC 7617 2.1](https://tools.ietf.org/html/rfc7617#section-2.1)).
  By default, ``charset`` is ``false``.
* ``conditions``: conditions under which the authentication protocol is
  to be executed.

If the ``conditions`` array is present, then it MUST have at least one
element, and each element must specify at least these two fields:

* ``comparand`` (string): either ``req.url`` or ``req.http.$HEADER``,
  where ``$HEADER`` is the name of a client request header.

* ``value`` (string): the value against which the ``comparand``
  is compared

``conditions`` may also have this optional field:

* ``compare``: one of the following (default ``equal``):

    * ``equal`` for string equality

    * ``not-equal`` for string inequality

    * ``match`` for regex match

    * ``not-match`` for regex non-match

If ``compare`` is ``equal`` or ``not-equal``, then ``value`` is
interpreted as a fixed string, and ``comparand`` is tested for
(in)equality with ``value``. Otherwise, ``value`` is interpreted as a
regular expression, and the ``comparand`` is tested for
(non-)match. Regexen are implemented as
[VCL regular expressions](https://varnish-cache.org/docs/6.1/reference/vcl.html#regular-expressions),
and hence have the syntax and semantics of
[PCRE](https://www.pcre.org/original/doc/html/).

The authentication protocol is executed only if all of the
``conditions`` succeed; in other words, the ``conditions`` are the
boolean AND of all of the match terms.

For example, these ``conditions`` specify that authentication is
executed when the URL begins with "/tea", and the Host header is
exactly "cafe.example.com":

```
      conditions:
      - comparand: req.url
        compare: match
        value: ^/tea(/|$)
      - comparand: req.http.Host
        value: cafe.example.com
```

Validation for ``VarnishConfig`` reports errors at apply time if:

* the ``auth`` array is empty
* either of the fields ``realm`` or ``secretName`` is left out
* any of the string fields are empty
* ``type`` has an illegal value (neither of ``basic`` or ``proxy``)

Other errors, in particular illegal regex syntax for ``conditions``,
are not reported until VCL load time. Check the controller log and
Events generated for the Varnish Service for error messages from the
VCL compiler.

Examples:
```
spec:
  # Require Basic Authentication for both the coffee and tea Services.
  auth:
    # For the coffee Service, require authentication for the realm
    # "coffee" when the Host is "cafe.example.com" and the URL path
    # begins with "/coffee".  Username/password pairs are taken from
    # the Secret "coffee-creds" in the same namespace, and clients
    # are advised that they are encoded with UTF-8.
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

    # For the tea Service, require authentication for the realm "tea"
    # when the Host is "cafe.example.com" and the URL path begins with
    # "/tea", with usernames/passwords from the Secret
    # "tea-creds". Note that the "type" defaults to basic and can be
    # left out.
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
```
spec:
  # Require Proxy Authentication for the realm "ingress" for every
  # request, using usernames/passwords from the Secret "proxy-creds".
  auth:
    - realm: ingress
      secretName: proxy-creds
      type: proxy
```

See the [``examples/`` folder](/examples/authentication) for working
examples of authentication configurations.

### ``spec.acl``

The ``acl`` element is optional, and if present contains a non-empty
array of specifications of access control lists for whitelisting or
blacklisting requests by IP address.

For each element of ``acl``, the required fields are:

* ``name`` (string): unique among the names for acl specifications

* ``addrs``: non-empty array of IP specifications (detailed below)

Optional fields for ``acl`` are:

* ``type``: ``whitelist`` or ``blacklist``, default ``whitelist``

* ``fail-status`` (integer): HTTP status for a synthetic failure
  response, default 403 (for "403 Forbidden")

    * If ``fail-status`` is < 100, then no failure response is
      generated as a result of ACL failure. The purpose is to bring
      about other effects, in particular setting a header with
      ``result-header`` as specified below. See the
      [``examples`` folder](/examples/authentication/) for a sample
      config in which this is used for "either-or" authorization --
      either an IP whitelist match or Basic Auth.

* ``comparand``: specification of the IP value against which the ACL
  is matched, as detailed below; default ``client.ip``

* ``conditions``: array of conditions under which the ACL match is
  executed. The ``conditions`` field has the same syntax and semantics
  as specified above for ``spec.auth`` (Basic and Proxy
  Authentication).  By default, ``conditions`` is empty, in which case
  the match is executed for every client request.

* ``result-header``: specifies a client request header and values to
  set for the header when the failure status is or is not invoked for
  the ACL evaluation. By default, no header is set.

Each element of the ``addrs`` array may have these fields, of which
``addr`` is required:

* ``addr`` (string, required): an IPv4 address, IPv6 address, or host
  name that is resolved by Varnish to an IP address at VCL load time

* ``mask-bits`` (integer, >= 0 and <= 128): bitmask as expressed by
  [CIDR notation](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing),
  so that an address range is specified; that is, the number of
  leading 1-bits in a subnet mask. By default (when ``mask-bits``
  is left out), there is no bitmask, and ``addr`` defines an exact
  IP address.

* ``negate`` (boolean): when true, the ACL does not match an IP
  if it matches the address or range specified by ``addr`` and
  ``mask-bits``; default false.

So the range 192.0.2.0/24 is expressed as:

```
      - addr: 192.0.2.0
        mask-bits: 24
```

``negate`` can be used to define IPs as "exceptions" if they fall in
ranges specified by other elements of the ACL:

```
      # Match all IPs in 192.0.2.0/24, but not 192.0.2.23.
      - addr: 192.0.2.0
        mask-bits: 24
      - addr: 192.0.2.23
        negate: true
```

When ``type`` is ``whitelist``, the failure response is sent when an
IP does not match the ACL. For ``blacklist``, the failure response is
invoked for an IP that does match the ACL.

``comparand`` (the "thing to be compared") specifies the IP value
against which the ACL is matched, and can have one of these values:

* ``client.ip``: interpreted as in
  [VCL](https://varnish-cache.org/docs/6.1/reference/vcl.html#local-server-remote-and-client)

* ``server.ip``: as in VCL

* ``remote.ip``: as in VCL

* ``local.ip``: as in VCL

* ``req.http.$HEADER``, where ``$HEADER`` is the name of a client
  request header

* ``xff-first``: match the first comma-separated field in the
   ``X-Forwarded-For`` request header

* ``xff-2ndlast``: match the next-to-last comma-separated field in
   ``X-Forwarded-For``, *after* Varnish has appended the client IP

To briefly summarize the ``*.ip`` values:

* ``remote.ip`` is always the address of the peer connection (the
  component that sent the request to Varnish)

* ``local.ip`` is the "Varnish side" of the connection (the IP at
  which the listener received the request)

* If the [PROXY protocol](/docs/varnish-pod-template.md) is in use:

     * ``client.ip`` and ``server.ip`` are the addresses sent in the
       PROXY header.

* Otherwise:

     * ``client.ip`` and ``server.ip`` are equal to ``remote.ip`` and
       ``local.ip``, respectively.

If ``req.http.$HEADER`` is specified for ``comparand``, the ACL is
matched against the IP in the value of the header. If the value of the
header is not an IP address, or if the header is not present in the
request, then the match fails. This can be used for a setup in which a
forwarding component sends a client IP in a header such as
``X-Real-IP``.

If ``xff-first`` is specified for ``comparand``, then the ACL is
matched against the IP in the first comma-separated field of the
``X-Forwarded-For`` request header. If the value of that field is not
an IP address, the match fails. Varnish always appends the client IP
to ``X-Forwarded-For``, and creates the header with that value if it
is not present in the request as received. So if there is no
``X-Forwarded-For`` when Varnish receives the request, then the first
field is the client IP (and ``xff-first`` is the same as matching
against ``client.ip``).

If ``xff-2ndlast`` is specified for ``comparand``, then the ACL is
matched against the next-to-last field in ``X-Forwarded-For`` *after*
Varnish appends the client IP. In other words, it is matched against
the last field in ``X-Forwarded-For`` as received by Varnish. If
Varnish receives a request without ``X-Forwarded-For``, then there is
no next-to-last field, and the match fails.

``X-Forwarded-For`` may appear more than once in a request (as if they
are separate headers). If either of ``xff-first`` or ``xff-2ndlast``
is specified as the comparand for any ACL in the VarnishConfig, the
values of ``X-Forwarded-For`` are collected into a single
comma-separated header, in the order in which they appeared in the
request.

Thus if Varnish receives a request with this value of ``X-Forwarded-For``:

```
X-Forwarded-For: 192.0.2.47, 203.0.113.11
```

... then ``xff-first`` specifies a match against 192.0.2.47, and
``xff-2ndlast`` specifies a match against 203.0.113.11.

If ``conditions`` are specified for an ACL, they define restrictions
for executing the match; the ACL match is executed only if all of the
``conditions`` succeed. The ``conditions`` field for ACLs has the same
syntax and semantics as specified above for Basic and Proxy
Authentication.

The ``result-header`` field specifies a client request header that is
set with a value for the "fail" or "success" results of the ACL
evaluation. If ``result-header`` is present, then all three of its
fields are required:

* ``header``: a string of the form ``req.http.$HEADER``, for the
  client request header ``$HEADER``

* ``success``: a string that is assigned to the header if the failure
  status is not invoked. So for ``type:whitelist``, the ``success``
  string is assigned if the address matches the ACL; for
  ``type:blacklist``, it is assigned if the address does not match.

* ``failure``: a string that is assigned to the header if the failure
  status is invoked.

The ``result-header`` makes it possible to check the result of the ACL
match at a later stage of request processing, for example to implement
logic that depends on the result.

See the [``examples/`` folder](/examples/acl) for working examples
of ACL configurations.

## ``spec.vcl``

The ``vcl`` element is optional, and if present contains a non-empty
string that is appended "as-is" to VCL code generated by the
controller that is loaded by Varnish instances. This provides a means
to write custom
[VCL](https://varnish-cache.org/docs/6.1/reference/vcl.html). For
example:

```
  vcl: |
    sub vcl_deliver {
    	set resp.http.Hello = "world";
    }

    sub vcl_backend_response {
    	set beresp.http.Backend = beresp.backend.name;
    }
```

Custom VCL currently cannot be validated at apply time. If the VCL is
invalid, the controller receives error messages from the compiler when
it attempts to load the code. Check the Varnish Service for
[Events](/docs/monitor.md) at the Warning level and the controller log
for error messages. Varnish usually returns status 106 for invalid
VCL.

See the [docs](/docs/custom-vcl.md) for conventions and restrictions
that apply to custom VCL, and for links to more information about VCL.

## ``spec.rewrites``

The ``rewrites`` element is optional, and if present contains a
non-empty array of objects specifying rewrites for URL paths or HTTP
headers. Headers may also be specified for deletion. Rewrites or
deletes may be unconditional, or may be only executed when certain
conditions are met, such as regular expression or string equality
matches. For regex matches, contents of the rewritten string may
result in part from backreferences to captured substrings in the
matching portion of the string.

See the [examples folder](/examples/rewrite) for working examples of
rewrites.

Elements of the ``rewrites`` array may have these fields, of which
``target`` and ``method`` are required:

* ``target`` (required): the object to which the rewritten string is
  assigned, or the object that is deleted. One of:

    * ``req.url``: the client request URL path (not permitted for
      deletes)

    * ``bereq.url``: the backend request URL path (not permitted for
      deletes)

    * ``req.http.$HEADER``: the client request header ``$HEADER``. For
      example ``req.http.Cookie`` for the Cookie request header

    * ``resp.http.$HEADER``: the client response header ``$HEADER``

    * ``bereq.http.$HEADER``: the backend request header ``$HEADER``

    * ``beresp.http.$HEADER``: the backend response header ``$HEADER``

    If the ``target`` specifies a header that is not present in the
    request or response, then the header is added with the new value.

* ``source``: the object against which matching or equality
  comparisons are executed, and from which strings may be extracted
  (for example as backreferences after regex matches). ``source`` has
  the same legal values as ``target``; that is, it may be the client
  or backend request URL path, or a client or backend request or
  response header, using the same notation shown above.

    * If ``source`` is left out, then it is the same as the
      ``target``.  In that case, the ``target`` object is edited in
      place.

    * ``target`` and ``source`` MUST specify either both client or
      both backend context. In other words, both of them must begin
      with either ``re`` or ``be``.

* ``rules``: a non-empty array of rules specifying conditions under
  which a rewrite is executed, and strings used for the rewrites. If
  the ``rules`` array is left out, then the rewrite is
  unconditional. Each rule is an object with these two fields:

    * ``value``: a string or pattern against which the ``source`` is
      compared. If the comparison succeeds for a ``value`` in the
      ``rules`` array, then that element of the array represents the
      rule to be applied, and the corresponding ``rewrite`` field is
      the string to be used for the rewrite. If ``value`` is a regular
      expression, then it has the syntax and semantics of
      [RE2](https://github.com/google/re2/wiki/Syntax). Each pattern
      or string in the ``value`` field MUST be unique in a ``rules``
      array; in other words, the same ``value`` MAY NOT appear more
      than once in ``rules``.

    * ``rewrite``: a string to be used for the rewrite if the
      corresponding ``value`` compares successfully with the
      ``source``.  The value of ``rewrite`` may be a fixed string, or
      it may contain backreferences to captured substrings after a
      regex match, depending on the choices for ``method`` and
      ``compare``, as described below.

    * If there is exactly one element of ``rules``, then the ``value``
      field may be left out for that element. In that case, the
      rewrite is applied unconditionally, and the value of the
      ``rewrite`` field is the string to be used. ``value`` is
      required when the ``method`` field is ``sub``, ``suball`` or
      ``rewrite`` (see below).

    * The ``rewrite`` field is required for each element of ``rules``,
      unless the ``method`` field is set to ``delete`` (see below).
      For ``delete``, the ``rewrite`` field is ignored and may be left
      out.

* ``method`` (required): an enum specifying the process by which the
  ``target`` is modified. One of:

    * ``replace``: a new string is assigned to the ``target``.  Any
      previous value of the ``target`` is overwritten.

    * ``sub`` (only permitted with a regular expression match):
      ``target`` is assigned the result of substituting the first
      matching substring of ``source`` with the value of ``rewrite``
      for the matching element of the ``rules`` array. That is, if
      the match succeeds for a ``value`` in ``rules``, then a
      substitution is executed using the corresponding ``rewrite``.
      The ``rewrite`` string may contain backreferences ``\1`` to
      ``\9``, referring to captured substrings in the match.

    * ``suball`` (only regex matches): like ``sub``, but the
      substitution is executed for each non-overlapping matching
      substring in ``source``. As with ``sub``, the value of the
      ``rewrite`` field may contain backreferences.

    * ``rewrite`` (only regex matches): for the matching element of
      ``rules`` (i.e. the element for which ``source`` matches
      successfully with ``value``), assign the value of the
      ``rewrite`` field to the ``target``. The ``rewrite`` field may
      contain backreferences to captured substrings. Non-matched and
      non-captured portions of the ``source`` string are ignored.

    * ``append``: concatenate a string after another string and write
      the result to the ``target``. The rules for determining the
      strings to be concatenated are described below.

    * ``prepend``: concatenate a string before another string and
      write the result to the ``target``, according to the rules
      described below.

    * ``delete``: delete the header specified in the ``target``.  For
      ``delete``, the ``target`` MAY NOT be ``req.url`` or
      ``bereq.url`` (you cannot delete a URL). If there is a ``rules``
      array, then the ``rewrite`` fields are ignored, but ``value``
      fields can be used to specify patterns or strings to be matched
      for conditional deletes.

    * If ``method`` is any of ``sub``, ``suball`` or ``rewrite``, then:

        * the ``rules`` array must be specified

        * a ``value`` field is required for each element of ``rules``,

        * each ``value`` is interpreted as a regular expression

        * the value of ``compare`` must be ``match`` (see below)

        This requirement can be summarized as: if the ``method``
        permits backreferences for the rewrite, then regex matching
        MUST be applied, and patterns for the match MUST be specified.

    * If there is exactly one element of ``rules`` and its ``value``
      field is left out, then the ``rewrite`` field of the rule is a
      fixed string to be used unconditionally in the rewrite:

        * For ``replace``, the string is written to the ``target``.

        * For ``append``, the string is concatenated after the
          ``source``, and the result is written to the ``target``.

        * For ``prepend``, the string is concatenated before the
          ``source``, and the result is written to the target.

        * ``sub``, ``suball``, ``rewrite`` and ``delete`` are not
          permitted (these require a ``value`` for each rule).

    * If there is no ``rules`` array, then the rewrite is applied
      unconditionally to the ``target`` and ``source``:

        * For ``replace``, the value of the ``source`` is written to
          the ``target`` (for example to copy one header to another).

        * For ``delete``, the ``target`` is deleted (and ``source`` is
          ignored).

        * For ``append``, the ``source`` is concatenated after the
          ``target``, and the result is written to the ``target``.

        * For ``prepend``, the ``source`` is concatenated before the
          ``target``.

        * ``sub``, ``suball`` and ``rewrite`` are not permitted (these
          require a ``rules`` array).

* ``compare``: an enum specifying the comparison operation used to
  compare the ``source`` with the ``value`` fields in the ``rules``.
  One of:

    * ``match`` (default): regular expression match. The match has the
      semantics of [RE2](https://github.com/google/re2/), and the
      ``value`` fields in the ``rules`` specify regular expressions.

    * ``equal``: string equality. The ``value`` fields specify fixed
      strings, and the corresponding rule applies if the ``source`` is
      exactly equal to ``value``. Characters such as wildcards or
      regex metacharacters are matched literally, and have no special
      meaning.

    * ``prefix``: fixed prefix match. The ``value`` fields specify
      fixed strings, and the corresponding rule applies if the
      ``source`` has a prefix that is exactly equal to ``value``.

* ``select``: when ``compare`` is ``match`` or ``prefix`` (regex or
  prefix match), this enum determines which rule is applied if more
  than one of them compares successfully, or imposes restrictions
  on multiple matches.

    For example, if ``compare`` is ``prefix`` and the ``rules``
    include ``/foo`` and ``/foo/bar`` in the ``value`` fields, then
    the string ``/foo/bar`` and ``/foo/bar/baz`` matches both of
    them. The ``select`` field specifies whether this is permitted,
    and if so which rewrite rule applies.

    Legal values of ``select`` depend on whether ``compare`` is set to
    ``match`` or ``prefix``. For both of them, ``select`` may be one
    of:

    * ``unique`` (default): only unique matches are permitted. If more
      than one rule compares successfully, then VCL failure is invoked
      (see below).

    * ``first``: if more than one rule compares successfully, then
      apply the rewrite for the first succeeding rule, in the order of
      the ``rules`` array.

    * ``last``: if more than one rule compares successfully, then
      apply the rewrite for the last succeeding rule in the ``rules``
      array.

    If ``compare`` is set to ``prefix``, then ``select`` may also be
    one of:

    * ``exact``: if more than one rule compares successfully, but the
      ``value`` for one of the rules is exactly equal to the
      ``source``, then apply the rewrite for that rule. If no rule
      matches exactly, then VCL failure is invoked (see below).

        For example, if:

          * ``compare`` is ``prefix``

          * there are two rules, with ``/foo`` and ``/foo/bar`` in the
            ``value`` fields

          * the ``source`` evaluates to ``/foo/bar``

        ... then the rule for ``/foo/bar`` is the exact match, and the
        rewrite specified for that rule applies. But no rules match
        exactly if ``source`` is ``/foo/bar/baz`` (although both of
        the rules specify a prefix for that string).

    * ``longest``: if more than one rule compares successfully, then
      apply the rewrite for the successful rule with the longest
      string in ``value``.

    * ``shortest``: if more than one rule compares successfully, then
      apply the rewrite for the successful rule with the shortest
      string in ``value``.

    If the conditions required by ``unique`` or ``exact`` are not met,
    then
    [VCL failure](https://varnish-cache.org/docs/6.1/users-guide/vcl-built-in-subs.html#common-return-keywords)
    is invoked after the comparison attempt. This means in most cases
    that a synthetic response with status 503 and the reason "VCL
    failed" is returned for the request.

* ``match-flags`` is an object with configuration to control comparison
  operations. If ``match-flags`` is absent, then comparisons are executed
  with default options.

    Only the ``case-sensitive`` field may be set if ``compare`` is
    ``equal`` or ``prefix``; all of the other fields are permitted
    only if ``compare`` is ``match``. In other words, case
    insensitivity can be specified for all comparison operations, but
    the other fields apply only to regex matching. The fields are:

    * ``case-sensitive`` (default ``true``): if ``false``, then regex
      and fixed-string comparisons are case insensitive.

    * ``anchor`` (default ``none``): sets anchoring at start-of-string
      or end-of-string for every pattern in the ``rules`` array;
      equivalent to using the ``^`` and ``$`` for start- and
      end-of-string in the notation for each pattern. Possible values
      are:

        * ``start``: each pattern is anchored at the start

        * ``both``: each pattern is anchored at both start and end.

        * ``none`` (default): no implicit anchoring (but ``^`` and/or
          ``$`` may be used in individual patterns)

    * ``literal`` (default ``false``): if ``true``, then the strings
      in the ``value`` fields of the ``rules`` are matched literally,
      with no special meaning for regex metacharacters.

    * ``never-capture`` (default ``false``): if ``true``, then
      substring capturing is not executed for regex matches. Consider
      setting ``never-capture`` to ``true`` if your patterns have
      round parentheses ``()`` for grouping only, and backreferences
      are not used in rewrite strings, since regex matches are faster
      without the captures.

    * ``utf8`` (default ``false``): if ``true``, then characters in
      each pattern match UTF8 code points; otherwise, the patterns and
      the strings to be matched are interpreted as Latin-1
      (ISO-8859-1). Note that characters in header values and URL
      paths almost always fall in the ASCII range, so the default is
      usually sufficient.

    * ``longest-match`` (default ``false``): if ``true``, then the
      matcher searches for the longest possible match where
      alternatives are possible. For example with the pattern
      ``a(b|bb)`` and the string ``abb``, ``abb`` matches when
      ``longest-match`` is ``true``, and backref 1 is
      ``bb``. Otherwise, ``ab`` matches, and backref 1 is ``b``.

    * ``posix-syntax`` (default ``false``): if ``true``, then patterns
      are restricted to POSIX (egrep) syntax. Otherwise, the full
      range of [RE2](https://github.com/google/re2/wiki/Syntax) is
      available. The next two flags (``perl-classes`` and
      ``word-boundary``) are only consulted when ``posix-syntax`` is
      ``true``.

    * ``perl-classes`` (default ``false``): if ``true`` and
      ``posix-syntax`` is also ``true``, then the perl character
      classes ``\d``, ``\s``, ``\w``, ``\D``, ``\S`` and ``\W`` are
      permitted in a pattern. When ``posix-syntax`` is ``false``, the
      perl classes are always permitted.

    * ``word-boundary`` (default ``false``): if ``true`` and
      ``posix-syntax`` is also ``true``, then the perl assertions
      ``\b`` and ``\B`` (word boundary and not a word boundary) are
      permitted in a pattern. When ``posix-syntax`` is ``false``, the
      word boundary assertions are always permitted.

    * ``max-mem`` (integer, default 8MB): an upper bound (in bytes)
      for the size of the compiled pattern. If ``max-mem`` is too
      small, the matcher may fall back to less efficient algorithms,
      or the pattern may fail to compile.

        This field very rarely needs to be set; the default is the RE2
        default, and is sufficient for typical patterns. Increasing
        ``max-mem`` is usually only necessary if VCL loads fail due to
        failed regex compiles, and the error message (shown in Event
        notifications and the controller log) indicates that the
        pattern is too large.

* ``vcl-sub`` is an enum indicating the
  [VCL subroutine](https://varnish-cache.org/docs/6.1/reference/states.html)
  in which the rewrite is executed; that is, the phase of request
  processing at which the rewrite is performed.

    If ``vcl-sub`` is left out, then the VCL subroutine for the
    rewrite is inferred from the values of ``target`` and ``source``:

    * If both of ``target`` and ``source`` are in ``req.*``
      (``req.url`` and/or ``req.http.$HEADER``), then the rewrite is
      executed in ``vcl_recv``; that is, just after client request
      headers are received.

    * If both of ``target`` and ``source`` are in ``bereq.*``
      (``bereq.url`` and/or ``bereq.http.$HEADER``), then the rewrite
      is executed in ``vcl_backend_fetch``; that is, just before a
      backend request is sent.

    * If at least one of ``source`` or ``target`` is in
      ``resp.http.*``, then the rewrite is executed in
      ``vcl_deliver``; that is, just before the client response is
      sent.

    * If at least one of ``source`` or ``target`` is in
      ``beresp.http.*``, then the rewrite is executed in
      ``vcl_backend_response``; that is, just after backend response
      headers are received.

    ``vcl-sub`` may be set explicitly with any of the following
    values, corresponding to the VCL subroutine with the ``vcl_``
    prefix (for example ``vcl_miss`` for ``vcl-sub:miss``):

    * ``recv``
    * ``pipe``
    * ``pass``
    * ``hash``
    * ``purge``
    * ``miss``
    * ``hit``
    * ``deliver``
    * ``synth``
    * ``backend_fetch``
    * ``backend_response``
    * ``backend_error``

    See the
    [VCL](https://varnish-cache.org/docs/6.1/reference/vcl.html)
    [docs](https://varnish-cache.org/docs/6.1/reference/states.html)
    for details.

    If more than one rewrite in the ``rewrites`` array specifies the
    same VCL subroutine, then they are executed in that subroutine in
    the order in which they appear in the array.
