#! /bin/bash -ex

function killforward {
    kill $KUBEPID
}

LOCALPORT=${LOCALPORT:-8888}

kubectl wait --timeout=2m pod -n kube-system -l app=varnish-ingress-controller \
        --for=condition=Ready

kubectl wait --timeout=2m pod -l app=varnish-ingress --for=condition=Initialized
sleep 1

kubectl port-forward svc/varnish-ingress ${LOCALPORT}:80 >/dev/null &
KUBEPID=$!
trap killforward EXIT

sleep 1
varnishtest ${TESTOPTS} -Dlocalport=${LOCALPORT} deploy.vtc
