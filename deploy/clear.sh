#! /bin/bash -ex

kubectl delete -f controller.yaml

kubectl delete -f backendcfg-crd.yaml

kubectl delete -f varnishcfg-crd.yaml

kubectl delete -f rbac.yaml

kubectl delete -f serviceaccount.yaml

kubectl wait --timeout=2m pod -n kube-system -l app=varnish-ingress-controller \
        --for=delete
