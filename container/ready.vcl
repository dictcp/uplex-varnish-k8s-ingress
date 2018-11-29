vcl 4.0;

include "bogo_backend.vcl";

# Send a synthetic response with status 200 to every request.
# Used for the readiness check when Varnish is ready.
sub vcl_recv {
        return(synth(200));
}
