#!/bin/bash

# delete webhook config
kubectl delete mutatingwebhookconfiguration mutating-webhook-demo-cfg
kubectl delete validatingwebhookconfiguration validating-webhook-demo-cfg

# delete webhook server deployment
kubectl delete deployment webhook-demo-deployment -n webhook-demo

# rebuild & push to hub
CURRENT_DIR=$(cd "$(dirname "$0")";pwd)
sh $CURRENT_DIR/build_image.sh

# deploy webhook server & webhook config
kubectl apply -f $CURRENT_DIR/../resources/deployment.yaml
kubectl apply -f $CURRENT_DIR/../resources/mutating-admission-webhook.yaml
kubectl apply -f $CURRENT_DIR/../resources/validating-admission-webhook.yaml
