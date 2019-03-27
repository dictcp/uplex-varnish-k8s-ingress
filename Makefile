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

all: k8s-ingress

vgo:
	go get golang.org/x/vgo

KUBEVER=kubernetes-1.9.11
install-code-gen:
	vgo get k8s.io/code-generator/cmd/client-gen@$(KUBEVER)
	vgo get k8s.io/code-generator/cmd/deepcopy-gen@$(KUBEVER)
	vgo get k8s.io/code-generator/cmd/lister-gen@$(KUBEVER)
	vgo get k8s.io/code-generator/cmd/informer-gen@$(KUBEVER)

CODE_SUBDIRS=./pkg/... ./cmd/...
build: vgo
	vgo fmt $(CODE_SUBDIRS)
	vgo generate $(CODE_SUBDIRS)
	vgo build $(CODE_SUBDIRS)

GENVER=code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1
BOILERPLATE=hack/boilerplate.txt
CLIENTPKG=code.uplex.de/uplex-varnish/k8s-ingress/pkg/client
CLIENTSET=$(CLIENTPKG)/clientset
LISTERS=$(CLIENTPKG)/listers
PKGMACHINERY=k8s.io/apimachinery/pkg
INPUTDIRS=$(PKGMACHINERY)/fields,$(PKGMACHINERY)/labels,$(PKGMACHINERY)/watch

generate: install-code-gen
	deepcopy-gen -i $(GENVER) -O zz_generated.deepcopy \
		--bounding-dirs $(GENVER) -h $(BOILERPLATE)
	lister-gen -i $(GENVER) --output-package $(LISTERS) \
		-h $(BOILERPLATE)
	client-gen --clientset-name versioned --input-base "" -i $(INPUTDIRS) \
		--input $(GENVER) --output-package $(CLIENTSET) \
		--clientset-path $(CLIENTSET) -h $(BOILERPLATE)
	informer-gen -i $(GENVER) \
		--versioned-clientset-package $(CLIENTSET)/versioned \
		--listers-package $(LISTERS) \
		--output-package $(CLIENTPKG)/informers -h $(BOILERPLATE)

k8s-ingress: build
	CGO_ENABLED=0 GOOS=linux vgo build -o k8s-ingress cmd/*.go

check: build
	golint ./pkg/controller/...
	golint ./pkg/interfaces/...
	golint ./pkg/varnish/...
	golint ./cmd/...
	vgo test -v ./pkg/controller/... ./pkg/interfaces/... ./pkg/varnish/...

test: check

clean:
	vgo clean $(CODE_SUBDIRS)
	rm -f cmd/main_version.go
	rm -f k8s-ingress
