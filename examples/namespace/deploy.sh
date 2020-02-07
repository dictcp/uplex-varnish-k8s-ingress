#! /bin/bash -ex

kubectl apply -f ns-and-sa.yaml

kubectl apply -f rbac.yaml

kubectl apply -f adm-secret.yaml

kubectl apply -f varnish.yaml

kubectl apply -f nodeport.yaml

kubectl apply -f controller.yaml

kubectl apply -f cafe.yaml

kubectl apply -f cafe-ingress.yaml
