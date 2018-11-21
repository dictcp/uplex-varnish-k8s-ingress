# Deploying an Ingress

There is a variety of ways to deploy an Ingress in a Kubernetes
cluster. The YAML configurations in this folder prepare a simple
method of deployment, suitable for testing and editing according to
your needs.

1. Define the Namespace ``varnish-ingress``, and a ServiceAccount
   named ``varnish-ingress`` in that namespace (``ns-and-sa.yaml``).
2. Apply [Role-based access
   control](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
   (RBAC) by creating a ClusterRole named ``varnish-ingress`` that
   permits the necessary API access for the Ingress controller, and a
   ClusterRoleBinding that assigns the ClusterRole to the
   ServiceAccount defined in the first step (``rbac.yaml``).
3. Define a Deployment named ``varnish-ingress`` in the namespace,
   associated with the ServiceAccount. Among other things, this
   identifies the container deployed as the Ingress, establishes an
   [imagePullPolicy](https://kubernetes.io/docs/concepts/containers/images/),
   defines the port number at which Varnish accepts requests, and so forth
   (``varnish-ingress.yaml``).
4. Define a NodePort as simple solution for directing requests to Varnish --
   an external port number assigned by the Kubernetes cluster through
   which the Varnish listener is accessed (``nodeport.yaml``).

This sequence of commands creates the resources described above:
```
$ kubectl apply -f ns-and-sa.yaml
$ kubectl apply -f rbac.yaml
$ kubectl apply -f varnish-ingress.yaml
$ kubectl apply -f nodeport.yaml
```
When these commands succeed:

* Varnish is started, and when the child process is running, it can
  receive requests sent to the external port established by the
  NodePort.
* The Ingress controller begins discovering Ingress definitions for
  the namespace of the Pod in which it is running (``varnish-ingress``
  in this example). Once it has obtained an Ingress definition, it
  creates a VCL configuration to implement it.
* Before such a VCL configuration is loaded, Varnish answers every
  request with a synthetic 404 Not Found response.

The [``examples/``](/examples) folder of the repository contains YAML
configurations for sample Services and an Ingress to test and
demonstrate the Ingress implementation.
