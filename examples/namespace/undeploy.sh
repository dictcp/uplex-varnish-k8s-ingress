#! /bin/bash -ex

kubectl delete -f cafe-ingress.yaml

kubectl delete -f cafe.yaml

kubectl delete -f controller.yaml

kubectl delete -f nodeport.yaml

kubectl delete -f varnish.yaml

kubectl delete -f adm-secret.yaml

kubectl delete -f rbac.yaml

kubectl delete -f ns-and-sa.yaml
