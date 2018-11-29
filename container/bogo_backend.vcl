# Bogus backend, to which no request is ever sent.
# - Sentinel that no backend was determined after a request has been
#   evaluated according to IngressRules.
# - Defined when the Ingress is not ready (not currently implementing
#   any IngressSpec), so that Varnish doesn't complain about no
#   backend definition.
backend notfound {
	# 192.0.2.0/24 reserved for docs & examples (RFC5737).
	.host = "192.0.2.255";
	.port = "80";
}
