#!/bin/bash

set -x
set -e


NAMESPACE=management-console
DOMAIN_NAME=test.test
#DOMAIN_NAME=asoc.pro


#kubectl create ns $NAMESPACE
kubectl apply -n $NAMESPACE -f deployment_postgresql.yaml
#kubectl create -n $NAMESPACE deployment wetty --image=wettyoss/wetty -- wetty --ssh-host=server
kubectl apply -n $NAMESPACE -f deployment_postgrest.yaml
kubectl expose -n $NAMESPACE deploy postgrest --port 3000
kubectl expose -n $NAMESPACE deploy postgresql --port 5432


kubectl create -f https://github.com/grafana/grafana-operator/releases/latest/download/kustomize-cluster_scoped.yaml


