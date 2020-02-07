#! /bin/bash -ex

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait -n varnish-ingress --timeout=2m pod -l app=varnish-ingress --for=condition=Ready

kubectl port-forward -n varnish-ingress svc/varnish-ingress ${LOCALPORT}:80 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} cafe.vtc
