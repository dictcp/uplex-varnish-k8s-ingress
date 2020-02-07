#! /bin/bash -ex

kubectl delete -f nodeport.yaml

kubectl delete -f varnish.yaml

kubectl delete -f adm-secret.yaml

echo Waiting until varnish-ingress Pods are deleted

kubectl wait --timeout=2m pod -l app=varnish-ingress --for=delete
