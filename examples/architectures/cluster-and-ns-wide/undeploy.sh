#! /bin/bash -ex

kubectl delete -f other-ingress.yaml

kubectl delete -f tea-ingress.yaml

kubectl delete -f coffee-ingress.yaml

kubectl delete -f varnish-coffee.yaml

kubectl delete -f nodeport-coffee.yaml

kubectl delete -f adm-secret-coffee.yaml

kubectl delete -f varnish-system.yaml

kubectl wait --timeout=2m pod -l app=varnish-ingress -n kube-system \
        --for=delete

kubectl delete -f nodeport-system.yaml

kubectl delete -f adm-secret-system.yaml

kubectl delete -f other.yaml

kubectl delete -f tea.yaml

kubectl delete -f coffee.yaml

kubectl delete -f namespace.yaml
