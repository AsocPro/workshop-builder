#!/bin/bash

set -x
set -e


NAMESPACE=management-console
DOMAIN_NAME=test.test
#DOMAIN_NAME=asoc.pro


kubectl create ns $NAMESPACE
kubectl create -n $NAMESPACE service clusterip  pg-svc --tcp=5432:5432

kubectl create -f https://github.com/grafana/grafana-operator/releases/latest/download/kustomize-cluster_scoped.yaml

helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm install -n grafana --values loki-values.yaml loki grafana/loki
helm install -n grafana --values promtail-values.yaml promtail grafana/promtail

kubectl apply -n grafana -f loki-grafana-datasource.yaml 
kubectl apply -f grafana-dashboard.yaml -n grafana

