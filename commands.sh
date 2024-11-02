#!/bin/bash

set -x
set -e


NAMESPACE=$1
DOMAIN_NAME=test.test
#DOMAIN_NAME=asoc.pro

cp ingress.yaml ${NAMESPACE}_ingress.yaml
sed -i "s/%%NAMESPACE%%/${NAMESPACE}/g" ${NAMESPACE}_ingress.yaml
sed -i "s/%%DOMAIN_NAME%%/$DOMAIN_NAME/g" ${NAMESPACE}_ingress.yaml

kubectl create ns $NAMESPACE
kubectl apply -n $NAMESPACE -f deployment_wetty.yaml
kubectl create -n $NAMESPACE service clusterip  wettysvc --tcp=3000:3000
kubectl apply -n $NAMESPACE -f ${NAMESPACE}_ingress.yaml
kubectl expose -n $NAMESPACE deploy wetty --port 3000


cp pv.yaml ${NAMESPACE}_pv.yaml
sed -i "s/%%NAMESPACE%%/${NAMESPACE}/g" ${NAMESPACE}_pv.yaml
kubectl apply -n ${NAMESPACE} -f ${NAMESPACE}_pv.yaml

cp pvc.yaml ${NAMESPACE}_pvc.yaml
sed -i "s/%%NAMESPACE%%/${NAMESPACE}/g" ${NAMESPACE}_pvc.yaml
kubectl apply -n ${NAMESPACE} -f ${NAMESPACE}_pvc.yaml



kubectl apply -n $NAMESPACE -f deployment_server.yaml
kubectl expose -n $NAMESPACE deployment server --port 22

rm ${NAMESPACE}_ingress.yaml
rm ${NAMESPACE}_pv.yaml
rm ${NAMESPACE}_pvc.yaml
