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
* ``condition``: conditions under which the authentication protocol is
  to be executed.

If the ``condition`` object is present, it may have either or both of
these fields:

* ``url-match`` (regular expression): pattern to match against the
  URL path of the request
* ``host-match`` (regular expression): pattern to match against the
  ``Host`` request header

If either or both of these two fields are present, then the
authentication protocol is executed for matching requests. If the
``condition`` is left out, then the authentication is required for
every client request.  The patterns in ``url-match`` and
``host-match`` are implemented as
[VCL regular expressions](https://varnish-cache.org/docs/6.1/reference/vcl.html#regular-expressions),
and hence have the syntax and semantics of
[PCRE](https://www.pcre.org/original/doc/html/).

Validation for ``VarnishConfig`` reports errors at apply time if:

* the ``auth`` array is empty
* either of the fields ``realm`` or ``secretName`` is left out
* any of the string fields are empty
* ``type`` has an illegal value (neither of ``basic`` or ``proxy``)

Other errors, in particular illegal regex syntax for ``url-match`` or
``host-match``, are not reported until VCL load time. Check the
controller log and Events generated for the Varnish Service for error
messages from the VCL compiler.

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
      condition:
        host-match: ^cafe\.example\.com$
        url-match: ^/coffee($|/)

    # For the tea Service, require authentication for the realm "tea"
    # when the Host is "cafe.example.com" and the URL path begins with
    # "/tea", with usernames/passwords from the Secret
    # "tea-creds". Note that the "type" defaults to basic and can be
    # left out.
    - realm: tea
      secretName: tea-creds
      condition:
        host-match: ^cafe\.example\.com$
        url-match: ^/tea($|/)
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

* ``comparand``: specification of the IP value against which the ACL
  is matched, as detailed below; default ``client.ip``

* ``conditions``: array of conditions under which the ACL match is
  executed, as detailed below. By default, ``conditions`` is empty,
  in which case the match is executed for every client request.

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

``comparand`` specifies the IP value against which the ACL is matched,
and can have one of these values:

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
for executing the match. Each element of ``conditions`` must specify
these three fields (all required):

* ``comparand`` (string): either ``req.url`` or ``req.http.$HEADER``,
  where ``$HEADER`` is the name of a client request header.

* ``regex``: a regular expression

* ``match`` (boolean): whether the condition term succeeds if the
  ``comparand`` does or does not match ``regex`` -- ``true`` for
  match, ``false`` for no-match.

The ACL match is executed only if all of the ``conditions`` succeed;
in other words, the ``conditions`` are the boolean AND of all of the
match terms.

For example, these ``conditions`` specify that the match is executed
when the URL begins with "/tea", and the Host header is exactly
"cafe.example.com":

```
      conditions:
      - comparand: req.url
        match: true
        regex: ^/tea(/|$)
      - comparand: req.http.Host
        match: yes
        regex: ^cafe\.example\.com$
```

See the [``examples/`` folder``](/examples/cal) for working examples
of ACL configurations.
