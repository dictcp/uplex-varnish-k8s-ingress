# Example configurations for Varnish Ingress

Subfolders in this folder contain sample manifests (YAMLs) to deploy
the Varnish implementation of Ingress, and demonstrate some of its
features. You may re-use and edit the configurations to fit your
requirements.

* [The "cafe" example](/examples/hello), a "hello world" for Ingress.

* Limiting the Ingress controller to
  [a single namespace](/examples/namespace).

* [Customizing the Pod template](/examples/varnish_pod_template)
  for Varnish

* [Self-sharding Varnish cluster](/examples/self-sharding)
  ([docs](/docs/self-sharding.md))

* [Basic and Proxy Authentication](/examples/authentication)

* [Access control lists](/examples/acl) -- whitelisting or
  blacklisting requests by IP address
