vcl 4.0;

include "bogo_backend.vcl";

# Send a synthetic response with status 503 to every request.
# Used for the readiness check and regular traffic when Varnish is not ready.
sub vcl_recv {
        return(synth(503));
}
