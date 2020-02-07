#! /bin/bash -ex

kubectl delete -f purge-method.yaml

kubectl delete -f ../hello/cafe-ingress.yaml

kubectl delete -f ../hello/cafe.yaml

echo "Waiting until varnish-ingress Pods are not ready"

N=0
until [ $N -ge 120 ]
do
    if kubectl get pods -l app=varnish-ingress | grep -q ' 1/1'; then
        sleep 10
        N=$(( N + 10 ))
        continue
    fi
    exit 0
done
echo "Giving up"
exit 1
