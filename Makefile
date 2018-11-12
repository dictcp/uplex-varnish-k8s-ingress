# Copyright (c) 2018 UPLEX Nils Goroll Systemoptimierung
# All rights reserved
#
# Author: Geoffrey Simmons <geoffrey.simmons@uplex.de>
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
# 1. Redistributions of source code must retain the above copyright
#    notice, this list of conditions and the following disclaimer.
# 2. Redistributions in binary form must reproduce the above copyright
#    notice, this list of conditions and the following disclaimer in the
#    documentation and/or other materials provided with the distribution.
#
# THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
# ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED.  IN NO EVENT SHALL AUTHOR OR CONTRIBUTORS BE LIABLE
# FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
# DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
# OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
# HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
# LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
# OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
# SUCH DAMAGE.

all: push

IMAGE = varnish-ingress

DOCKER_BUILD_OPTIONS =

MINIKUBE =

PACKAGES = \
	k8s.io/client-go/kubernetes \
	k8s.io/client-go/kubernetes/scheme \
	k8s.io/client-go/kubernetes/typed/core/v1 \
	k8s.io/client-go/rest \
	k8s.io/client-go/tools/cache \
	k8s.io/client-go/tools/record \
	k8s.io/client-go/util/workqueue \
	k8s.io/api/core/v1 \
	k8s.io/api/extensions/v1beta1 \
	k8s.io/apimachinery/pkg/apis/meta/v1 \
	k8s.io/apimachinery/pkg/fields \
	k8s.io/apimachinery/pkg/labels \
	k8s.io/apimachinery/pkg/util/intstr \
	k8s.io/apimachinery/pkg/util/wait \
	code.uplex.de/uplex-varnish/varnishapi/pkg/admin \
	code.uplex.de/uplex-varnish/varnishapi/pkg/vsm

k8s-ingress:
	go get ${PACKAGES}
	go generate
	go fmt ./...
	GOOS=linux go build -o k8s-ingress *.go

check: k8s-ingress
	go vet ./...
	golint ./...
	go test -v ./...

test: check

docker-minikube:
ifeq ($(MINKUBE),1)
	eval $(minikube docker-env)
endif

container: check docker-minikube
	docker build $(DOCKER_BUILD_OPTIONS) -t $(IMAGE) .

push: docker-minikube container
	docker push $(IMAGE)

clean:
	go clean ./...
	rm main_version.go
