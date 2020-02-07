#! /bin/bash -x

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait --timeout=2m pod -l app=varnish-ingress,example!=cli-args --for=delete

set -e
kubectl wait --timeout=2m pod -l example=cli-args --for=condition=Ready

kubectl port-forward svc/varnish-ingress ${LOCALPORT}:80 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} cafe_cli-args.vtc
