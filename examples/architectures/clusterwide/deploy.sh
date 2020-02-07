#! /bin/bash -ex

# Delete the Varnish Service in namespace default.
# Otherwise the Service in kube-system is not unique in the cluster,
# and a Service for the Ingresses in the other namespaces cannot be
# determined.
kubectl delete -f ../../../deploy/nodeport.yaml

kubectl apply -f namespace.yaml

kubectl apply -f coffee.yaml

kubectl apply -f tea.yaml

kubectl apply -f other.yaml

kubectl apply -f adm-secret.yaml

kubectl apply -f nodeport.yaml

kubectl apply -f varnish.yaml

kubectl apply -f coffee-ingress.yaml

kubectl apply -f tea-ingress.yaml

kubectl apply -f other-ingress.yaml
