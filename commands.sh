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
#kubectl create -n $NAMESPACE deployment wetty --image=wettyoss/wetty -- wetty --ssh-host=server
kubectl create -n $NAMESPACE service clusterip  wettysvc --tcp=3000:3000
kubectl apply -n $NAMESPACE -f ${NAMESPACE}_ingress.yaml
kubectl expose -n $NAMESPACE deploy wetty --port 3000




kubectl apply -n $NAMESPACE -f deployment_server.yaml
#kubectl create -n $NAMESPACE deployment server --image=panubo/sshd
kubectl expose -n $NAMESPACE deployment server --port 22
