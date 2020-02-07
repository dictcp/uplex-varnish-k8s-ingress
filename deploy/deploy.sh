#! /bin/bash -ex

kubectl apply -f adm-secret.yaml

kubectl apply -f varnish.yaml

kubectl apply -f nodeport.yaml
