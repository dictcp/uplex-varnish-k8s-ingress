#! /bin/bash -ex

kubectl apply -f ../hello/cafe.yaml

kubectl apply -f ../hello/cafe-ingress.yaml

kubectl apply -f basic-secrets.yaml

kubectl apply -f acl-or-auth.yaml
