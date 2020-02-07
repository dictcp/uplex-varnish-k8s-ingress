#! /bin/bash -ex

kubectl delete -f tea-ingress.yaml

kubectl delete -f coffee-ingress.yaml

kubectl delete -f varnish-coffee.yaml

kubectl delete -f nodeport-coffee.yaml

kubectl delete -f adm-secret-coffee.yaml

kubectl delete -f varnish-tea.yaml

kubectl delete -f nodeport-tea.yaml

kubectl delete -f adm-secret-tea.yaml

kubectl delete -f tea.yaml

kubectl delete -f coffee.yaml

kubectl delete -f namespace.yaml

kubectl delete -f controller.yaml

kubectl wait --timeout=2m pod -l app=varnish-ingress-controller,example=coffee \
        -n kube-system --for=delete
