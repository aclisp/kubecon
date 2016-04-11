# build the docker image.

REPO=61.160.36.122:8080
PROJECT=default
APP=sigma-console
VERSION=1.0.0

IMAGE=${REPO}/${PROJECT}/${APP}:${VERSION}

all:
	godep go build -o kubecon main.go
	docker build -t ${IMAGE} .
	docker push ${IMAGE}

.PHONY: all
