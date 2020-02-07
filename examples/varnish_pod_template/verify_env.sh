#! /bin/bash -x

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait --timeout=2m pod -l app=varnish-ingress,example!=env --for=delete

set -e
kubectl wait --timeout=2m pod -l example=env --for=condition=Ready

kubectl port-forward svc/varnish-ingress ${LOCALPORT}:81 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} cafe_proxy.vtc
