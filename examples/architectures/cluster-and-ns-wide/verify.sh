#! /bin/bash -ex

function killsystem {
    kill $SYSTEMPID
}

function killcafe {
    kill $SYSTEMPID
    kill $CAFEPID
}

SYSTEMPORT=${SYSTEMPORT:-8888}
CAFEPORT=${CAFEPORT:-9999}

kubectl wait -n kube-system --timeout=2m pod -l app=varnish-ingress --for=condition=Ready

kubectl wait -n cafe --timeout=2m pod -l app=varnish-ingress --for=condition=Ready

kubectl port-forward -n kube-system svc/varnish-ingress ${SYSTEMPORT}:80 >/dev/null &
SYSTEMPID=$!
trap killsystem EXIT

kubectl port-forward -n cafe svc/varnish-ingress ${CAFEPORT}:80 >/dev/null &
CAFEPID=$!
trap killcafe EXIT

sleep 1
varnishtest ${TESTOPTS} -Dsystemport=${SYSTEMPORT} -Dcafeport=${CAFEPORT} cafe.vtc

# Parse the controller log for these lines (Ingress names in any order):
# Ingresses implemented by Varnish Service kube-system/varnish-ingress: [other/other-ingress cafe/tea-ingress]
# Ingresses implemented by Varnish Service cafe/varnish-ingress: [cafe/coffee-ingress]

# Get the name of the controller Pod
CTLPOD=$(kubectl get pods -n kube-system -l app=varnish-ingress-controller -o jsonpath={.items[0].metadata.name})

# Extract the last matching lines
SYSINGS=$(kubectl logs -n kube-system $CTLPOD | grep 'Ingresses implemented by Varnish Service kube-system/varnish-ingress' | tail -1)
CAFEINGS=$(kubectl logs -n kube-system $CTLPOD | grep 'Ingresses implemented by Varnish Service cafe/varnish-ingress' | tail -1)

# Check those line for the Ingress names
echo $SYSINGS | grep 'other/other-ingress'
echo $SYSINGS | grep 'cafe/tea-ingress'
echo $CAFEINGS | grep 'cafe/coffee-ingress'
