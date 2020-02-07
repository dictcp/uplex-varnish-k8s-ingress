#! /bin/bash -ex

kubectl apply -f ../hello/cafe.yaml

kubectl apply -f ../hello/cafe-ingress.yaml

kubectl apply -f pass-on-session-cookie.yaml
