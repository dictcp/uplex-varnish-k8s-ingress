# ``match-flags`` -- configuring match operations

This is the authoritative reference for the ``match-flags`` field,
which is used in a number of places in the custom resource
configurations where string comparisons are specified. For example in
the [``rewrites`` configuration](/docs/ref-varnish-cfg.md) for header
and URL rewriting, and in the [``req-disposition``
configuration](/docs/ref-req-disposition.md) to specify the
disposition of client requests.

Three types of string comparison are used by this Ingress
implementation:

* regular expression matching: strings are matched against patterns
  with the syntax and semantics of [RE2 regular
  expressions](https://github.com/google/re2/wiki/Syntax)

* fixed string matches: strings are tested for equality with literal
  string values. Characters such as wildcards or regular expression
  metacharacters have no special meaning.

* prefix matches: strings are tested as to whether they begin with any
  one of a set of strings. These are also literal matches against the
  prefixes, in the sense that no character in the prefix has a special
  meaning.

The string comparisons are typically configured for an array of
possible string values. The comparisons are evaluated as true if they
are true for any one of the strings in the array. In other words, the
string array represents a boolean OR of comparisons.

The ``match-flags`` object is used to configure and control the
comparison operations. ``match-flags`` is optional in every context in
which it may be specified -- if it is left out, then comparisons are
executed with default options.

Only the ``case-sensitive`` field may be set for fixed string and
prefix matches, typically configured with the string enum values
``equal`` and ``prefix``, respectively.  All of the other fields are
permitted only for regular expression matches, typically configured as
``match``. In other words, case insensitivity can be specified for all
comparison operations, but the other fields apply only to regex
matching.

The ``match-flag`` fields are adapted from the
[RE2](https://github.com/google/re2/) library (via [VMOD
re2](https://code.uplex.de/uplex-varnish/libvmod-re2)).  The fields
are:

* ``case-sensitive`` (default ``true``): if ``false``, then
  comparisons (regex, fixed string or prefix) are case insensitive.

* ``anchor`` (default ``none``): sets anchoring at start-of-string or
  end-of-string for every pattern to be matched; equivalent to using
  the ``^`` and ``$`` for start- and end-of-string in the notation for
  each pattern. Possible values are:

    * ``start``: each pattern is anchored at the start

    * ``both``: each pattern is anchored at both start and end.

    * ``none`` (default): no implicit anchoring (but ``^`` and/or
      ``$`` may be used in individual patterns)

* ``literal`` (default ``false``): if ``true``, then the strings to to
  be matched are matched literally, with no special meaning for regex
  metacharacters (despite the use of regex matching).

* ``never-capture`` (default ``false``): if ``true``, then substring
  capturing is not executed for regex matches. Capturing
  backreferences is necessary for some applications, such as header
  and URL rewrites.  But consider setting ``never-capture`` to
  ``true`` if your patterns have round parentheses ``()`` for grouping
  only, and backreferences are not needed, since regex matches are
  faster without the captures.

* ``utf8`` (default ``false``): if ``true``, then characters in each
  pattern match UTF8 code points; otherwise, the patterns and the
  strings to be matched are interpreted as Latin-1 (ISO-8859-1). Note
  that characters in header values and URL paths almost always fall in
  the ASCII range, so the default is usually sufficient. Note also that
  this differs from the default in the RE2 library.

* ``longest-match`` (default ``false``): if ``true``, then the matcher
  searches for the longest possible match where alternatives are
  possible. For example with the pattern ``a(b|bb)`` and the string
  ``abb``, ``abb`` matches when ``longest-match`` is ``true``, and
  backref 1 is ``bb``. Otherwise, ``ab`` matches, and backref 1 is
  ``b``.

* ``posix-syntax`` (default ``false``): if ``true``, then patterns are
  restricted to POSIX (egrep) syntax. Otherwise, the full range of
  [RE2](https://github.com/google/re2/wiki/Syntax) is available.

    The next two flags (``perl-classes`` and ``word-boundary``) are
    only consulted when ``posix-syntax`` is ``true``.

* ``perl-classes`` (default ``false``): if ``true`` and
  ``posix-syntax`` is also ``true``, then the perl character classes
  ``\d``, ``\s``, ``\w``, ``\D``, ``\S`` and ``\W`` are permitted in a
  pattern. When ``posix-syntax`` is ``false``, the perl classes are
  always permitted.

* ``word-boundary`` (default ``false``): if ``true`` and
  ``posix-syntax`` is also ``true``, then the perl assertions ``\b``
  and ``\B`` (word boundary and not a word boundary) are permitted in
  a pattern. When ``posix-syntax`` is ``false``, the word boundary
  assertions are always permitted.

* ``max-mem`` (integer, default 8MB): an upper bound (in bytes) for
  the size of the compiled pattern. If ``max-mem`` is too small, the
  matcher may fall back to less efficient algorithms, or the pattern
  may fail to compile.

    This field very rarely needs to be set; the default is the RE2
    default, and is sufficient for typical patterns. Increasing
    ``max-mem`` is usually only necessary if VCL loads fail due to
    failed regex compiles, and the error message (shown in Event
    notifications and the controller log) indicates that the pattern
    is too large.

See the [``examples/``](/examples/) folder, particular the examples
for [rewriting](/examples/rewrite) and [client request
disposition](/examples/req-disposition/), for working examples in
which the ``match-flags`` object is used.
