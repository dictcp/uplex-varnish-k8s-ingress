#! /bin/bash -ex

kubectl delete svc varnish-ingress

kubectl delete deploy varnish

echo Waiting until example varnish-ingress Pods are deleted
kubectl wait --timeout=2m pod -l app=varnish-ingress --for=delete

kubectl delete -f ../hello/cafe-ingress.yaml

kubectl delete -f ../hello/cafe.yaml

kubectl apply -f ../../deploy/varnish.yaml

kubectl apply -f ../../deploy/nodeport.yaml

echo Waiting until varnish-ingress Pods are running
kubectl wait --timeout=2m pod -l app=varnish-ingress --for=condition=Initialized
