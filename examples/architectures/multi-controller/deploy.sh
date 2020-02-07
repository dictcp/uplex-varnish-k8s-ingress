#! /bin/bash -ex

# Assumes that the kube-system controller as configured in deploy/ is
# already deployed.

kubectl apply -f controller.yaml

kubectl apply -f namespace.yaml

kubectl apply -f coffee.yaml

kubectl apply -f tea.yaml

kubectl apply -f adm-secret-tea.yaml

kubectl apply -f nodeport-tea.yaml

kubectl apply -f varnish-tea.yaml

kubectl apply -f adm-secret-coffee.yaml

kubectl apply -f nodeport-coffee.yaml

kubectl apply -f varnish-coffee.yaml

kubectl apply -f coffee-ingress.yaml

kubectl apply -f tea-ingress.yaml
