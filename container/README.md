# Container images for Varnish and the Ingress controller

Varnish instances to be deployed as realizations of Ingresses and the
controller that manages them are implemented in separate containers.
One controller is able to manage a group of Varnish instances, for
example when they are realized as a Deployment with several replicas.

The Dockerfiles and other files needed to build the two images are in
the current folder. The build commands are encapsulated by these
targets:
```
# Build the image for Varnish as an Ingress
$ make varnish

# Build the image for the controller
$ make controller

# Build both images
$ make
```
If you are testing with ``minikube``, set the environment variable
``MINIKUBE=1`` before running ``make container``, so that the
container will be available to the local k8s cluster:
```
$ MINIKUBE=1 make container
```
Both images must be pushed to a repository available to the k8s
cluster.

* The Varnish image is tagged ``varnish-ingress/varnish``.
* The controller image is tagged ``varnish-ingress/controller``.

The images are only suitable for the realization of Kubernetes
Ingresses.  Since the Varnish image has configurations specific for
this purpose, it is not suited as a general-purpose Varnish
deployment.

## Varnish image

The Varnish image currently runs Varnish version 6.1.1. The image runs
Varnish in the foreground as its entry point (``varnishd -F``, see
[``varnishd(1)``](https://varnish-cache.org/docs/6.0/reference/varnishd.html));
so the image runs the Varnish master process as PID 1, which in turn
controls the child or worker process that implements the HTTP proxy.

Varnish is live (although not necessarily ready) when the master
process is running, hence if the container is running at all. The
Deployment configuration for a Varnish instance illustrated in the
[``deploy/``](/deploy) folder shows a simple example of a k8s liveness
check.

Varnish runs with two listeners:

* for "regular" client requests.
* for readiness checks from the k8s cluster
    * The Varnish instance is ready when it is configured to respond
      with status 200 to requests for a specific URL received over the
      "readiness listener". The controller ensures that this happens
      after it has loaded the configuration for an Ingress at the
      instance. When it is not ready, it responds with status 503.

**TO DO**: The listeners are currently hard-wired at ports 80 and
8080, respectively.  It is presently not possible to specify the PROXY
protocol for a listener. The readiness check is hard-wired at the URL
path ``/ready``.

Another listener is opened to receive administrative commands (see
[``varnish-cli(7)``](https://varnish-cache.org/docs/6.0/reference/varnish-cli.html));
this connection will be used by the controller to manage the Varnish
instance.

**TO DO**: The admin port is currently hard-wired as port 6081.

Use of the administrative interface requires authorization based on a
secret that must be shared by the Varnish instances and the
controller. This must be deployed as a k8s Secret, whose contents are
in a file mounted to a path on each Varnish instance, and are obtained
by the controller from the cluster API. The configurations in the
[``deploy/``](/deploy) folder show how this is done.

**TO DO**: The path of the secret file on the Varnish instance is
currently hard-wired as ``/var/run/varnish/_.secret``.

The Varnish instance is configured to start with a start script that
does the following:

* load a VCL configuration that generates a synthetic 200 response for
  every request
* load a VCL that generates a synthetic 503 response response for
  every request
* apply a "readiness" label to the VCL configuration that responds
  with 503
* apply a "regular" label to the VCL that responds with 503
* load a "boot" VCL configuration that directs requests over the
  "readiness" listener to the "readiness" label, and all requests over
  the public HTTP port to the "regular" label
* make the "boot" configuration active
* start the child process

This means that in the initial configuration, Varnish responds with
the synthetic 503 response to all requests, received over both the
readiness port and the public HTTP port.

The controller operates by loading VCL configurations to implement
Ingress definitions, and swapping the labels. When the controller has
loaded a configuration for an Ingress, the "regular" label is applied
to it. It then applies the "readiness" label to the configuration that
leads to 200 responses; so a Varnish instance becomes ready after it
has successfully loaded its first configuration for an Ingress.

