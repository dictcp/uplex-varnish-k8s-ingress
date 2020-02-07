#! /bin/bash -ex

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait -n kube-system --timeout=2m pod -l app=varnish-ingress \
        --for=condition=Ready

kubectl port-forward -n kube-system svc/varnish-ingress ${LOCALPORT}:80 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} cafe.vtc

# Parse the controller log for this line (Ingress names in any order):
# Ingresses implemented by Varnish Service kube-system/varnish-ingress: [coffee/coffee-ingress tea/tea-ingress other/other-ingress]

# Get the name of the controller Pod
CTLPOD=$(kubectl get pods -n kube-system -l app=varnish-ingress-controller -o jsonpath={.items[0].metadata.name})

# Extract the last matching line
INGS=$(kubectl logs -n kube-system $CTLPOD | grep 'Ingresses implemented by Varnish Service kube-system/varnish-ingress' | tail -1)

# Check that line for the three Ingress names
echo $INGS | grep 'coffee/coffee-ingress'
echo $INGS | grep 'tea/tea-ingress'
echo $INGS | grep 'other/other-ingress'
