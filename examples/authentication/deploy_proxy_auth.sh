#! /bin/bash -ex

kubectl apply -f ../hello/cafe.yaml

kubectl apply -f ../hello/cafe-ingress.yaml

kubectl apply -f proxy-auth-secrets.yaml

kubectl apply -f proxy-auth.yaml
