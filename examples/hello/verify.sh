#! /bin/bash -ex

# Nothing special about self-sharding is verified here, just run the
# same tests as for the cafe example ("hello").

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait --timeout=2m pod -l app=varnish-ingress --for=condition=Ready

kubectl port-forward svc/varnish-ingress ${LOCALPORT}:80 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} cafe.vtc
