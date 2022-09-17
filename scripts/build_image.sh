#!/bin/bash

export DOCKER_USER=wannazjx
export GO111MODULE=auto
export GOPROXY=https://goproxy.cn,direct

CURRENT_DIR=$(cd "$(dirname "$0")";pwd)
#echo $CURRENT_DIR /root/go_project/webhook-demo/scripts
# build webhook
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o webhook-demo $CURRENT_DIR/../.
# build docker image
docker build --no-cache -f $CURRENT_DIR/Dockerfile -t ${DOCKER_USER}/webhook-demo:v1 .
rm -rf webhook-demo

docker push ${DOCKER_USER}/webhook-demo:v1
