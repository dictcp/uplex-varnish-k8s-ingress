#! /bin/bash -ex

kubectl apply -f namespace.yaml

kubectl apply -f coffee.yaml

kubectl apply -f tea.yaml

kubectl apply -f other.yaml

kubectl apply -f adm-secret-system.yaml

kubectl apply -f nodeport-system.yaml

kubectl apply -f varnish-system.yaml

kubectl apply -f adm-secret-coffee.yaml

kubectl apply -f nodeport-coffee.yaml

kubectl apply -f varnish-coffee.yaml

kubectl apply -f coffee-ingress.yaml

kubectl apply -f tea-ingress.yaml

kubectl apply -f other-ingress.yaml
