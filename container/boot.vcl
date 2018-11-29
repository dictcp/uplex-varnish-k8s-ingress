vcl 4.1;

include "bogo_backend.vcl";

sub vcl_recv {
	if (local.socket == "k8s") {
		if (req.url == "/ready") {
			return (vcl(vk8s_readiness));
		}
		return (synth(404));
	}
	return (vcl(vk8s_regular));
}
