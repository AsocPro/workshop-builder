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

kubectl create -n $NAMESPACE service clusterip  pg-svc --tcp=5432:5432

kubectl create -f https://github.com/grafana/grafana-operator/releases/latest/download/kustomize-cluster_scoped.yaml

# Minio is used for loki operator. I just used the helm chart to make things quicker.
#kubectl create namespace minio-tenant
#
#
#kubectl apply -n $NAMESPACE -f minio-pv.yaml
#
#kubectl kustomize github.com/minio/operator\?ref=v6.0.3 | kubectl apply -f -

helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm install -n grafana --values loki-values.yaml loki grafana/loki

kubectl apply -n grafana -f loki-grafana-datasource.yaml 

