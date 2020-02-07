#! /bin/bash -ex

kubectl delete -f other-ingress.yaml

kubectl delete -f tea-ingress.yaml

kubectl delete -f coffee-ingress.yaml

kubectl delete -f varnish.yaml

kubectl delete -f nodeport.yaml

kubectl delete -f adm-secret.yaml

kubectl delete -f other.yaml

kubectl delete -f tea.yaml

kubectl delete -f coffee.yaml

kubectl delete -f namespace.yaml

# Restores the Varnish Service in namespace default.
kubectl apply -f ../../../deploy/nodeport.yaml

echo Waiting until varnish-ingress Pods are running
kubectl wait --timeout=2m pod -l app=varnish-ingress --for=condition=Initialized
