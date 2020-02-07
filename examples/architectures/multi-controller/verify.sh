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

kubectl wait -n kube-system --timeout=2m pod -l app=varnish-ingress-controller \
        --for=condition=Ready

kubectl wait -n cafe --timeout=2m pod -l app=varnish-ingress \
        --for=condition=Ready

kubectl port-forward -n cafe svc/varnish-coffee ${COFFEEPORT}:80 >/dev/null &
COFFEEPID=$!
trap killcoffee EXIT

kubectl port-forward -n cafe svc/varnish-tea ${TEAPORT}:80 >/dev/null &
TEAPID=$!
trap killcafe EXIT

sleep 1
varnishtest ${TESTOPTS} -Dcoffeeport=${COFFEEPORT} -Dteaport=${TEAPORT} cafe.vtc

# Parse the tea controller log for these lines:
# Ingress class:varnish
# Ingress cafe/tea-ingress configured for Varnish Service cafe/varnish-tea
# Ignoring Ingress cafe/coffee-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish'

# Get the name of the tea controller Pod
CTLPOD=$(kubectl get pods -n kube-system -l app=varnish-ingress-controller,example!=coffee -o jsonpath={.items[0].metadata.name})

# Match the logs
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingress class:varnish' 
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingress cafe/tea-ingress configured for Varnish Service cafe/varnish-tea'
kubectl logs -n kube-system $CTLPOD | grep -q "Ignoring Ingress cafe/coffee-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish'"

# Parse the coffee controller log for these lines
# Ingress class:varnish-coffee
# Ingress cafe/coffee-ingress configured for Varnish Service cafe/varnish-coffee
# Ignoring Ingress cafe/tea-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish-coffee'

# Get the name of the tea controller Pod
CTLPOD=$(kubectl get pods -n kube-system -l app=varnish-ingress-controller -l example=coffee -o jsonpath={.items[0].metadata.name})

# Match the logs
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingress class:varnish-coffee' 
kubectl logs -n kube-system $CTLPOD | grep -q 'Ingress cafe/coffee-ingress configured for Varnish Service cafe/varnish-coffee'
kubectl logs -n kube-system $CTLPOD | grep -q "Ignoring Ingress cafe/tea-ingress, Annotation 'kubernetes.io/ingress.class' absent or is not 'varnish-coffee'"
