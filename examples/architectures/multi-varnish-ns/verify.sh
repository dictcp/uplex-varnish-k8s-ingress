#! /bin/bash -ex

function killcoffee {
    kill $COFFEEPID
}

function killcafe {
    kill $COFFEEPID
    kill $TEAPID
}

COFFEEPORT=${COFFEEPORT:-8888}
TEAPORT=${TEAPORT:-9999}

kubectl wait -n cafe --timeout=2m pod -l app=varnish-ingress --for=condition=Ready

kubectl port-forward -n cafe svc/varnish-coffee ${COFFEEPORT}:80 >/dev/null &
COFFEEPID=$!
trap killcoffee EXIT

kubectl port-forward -n cafe svc/varnish-tea ${TEAPORT}:80 >/dev/null &
TEAPID=$!
trap killcafe EXIT

sleep 1
varnishtest ${TESTOPTS} -Dcoffeeport=${COFFEEPORT} -Dteaport=${TEAPORT} cafe.vtc

# Parse the controller log for these lines
# Ingresses implemented by Varnish Service cafe/varnish-tea: [cafe/tea-ingress]
# Ingresses implemented by Varnish Service cafe/varnish-coffee: [cafe/coffee-ingress]

# Get the name of the controller Pod
CTLPOD=$(kubectl get pods -n kube-system -l app=varnish-ingress-controller -o jsonpath={.items[0].metadata.name})

# Match the logs
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingresses implemented by Varnish Service cafe/varnish-coffee: \[cafe/coffee-ingress\]' 
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingresses implemented by Varnish Service cafe/varnish-tea: \[cafe/tea-ingress\]' 
