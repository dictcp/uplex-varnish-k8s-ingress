#! /bin/bash -ex

kubectl create -f cafe.yaml

kubectl create -f cafe-ingress.yaml
