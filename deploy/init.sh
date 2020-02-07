#! /bin/bash -ex

kubectl apply -f serviceaccount.yaml

kubectl apply -f rbac.yaml

kubectl apply -f varnishcfg-crd.yaml

kubectl apply -f backendcfg-crd.yaml

kubectl apply -f controller.yaml
