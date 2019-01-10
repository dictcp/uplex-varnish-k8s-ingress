/*
 * Copyright (c) 2018 UPLEX Nils Goroll Systemoptimierung
 * All rights reserved
 *
 * Author: Geoffrey Simmons <geoffrey.simmons@uplex.de>
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

// Don't return error (requeuing the vcfg) if either of Ingresses or
// Services are not found -- they will sync as needed when and if they
// are discovered.
func (worker *NamespaceWorker) enqueueIngsForVcfg(
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	svc2ing := make(map[*api_v1.Service]*extensions.Ingress)
	ings, err := worker.ing.List(labels.Everything())
	if errors.IsNotFound(err) {
		worker.log.Infof("VarnishConfig %s/%s: no Ingresses found in "+
			"workspace %s", vcfg.Namespace, vcfg.Name,
			worker.namespace)
		return nil
	}
	if err != nil {
		return err
	}
	for _, ing := range ings {
		if !isVarnishIngress(ing) {
			continue
		}
		vSvc, err := worker.getVarnishSvcForIng(ing)
		if errors.IsNotFound(err) {
			worker.log.Infof("VarnishConfig %s/%s: no Varnish "+
				"Services found in workspace %s",
				vcfg.Namespace, vcfg.Name, worker.namespace)
			return nil
		}
		if err != nil {
			return err
		}
		if vSvc != nil {
			svc2ing[vSvc] = ing
		}
	}

	svcSet := make(map[string]struct{})
	for _, svc := range vcfg.Spec.Services {
		if _, exists := svcSet[svc]; exists {
			continue
		}
		svcSet[svc] = struct{}{}

		svcObj, err := worker.svc.Get(svc)
		if err != nil {
			return err
		}
		if ing, exists := svc2ing[svcObj]; exists {
			worker.log.Infof("VarnishConfig %s/%s: enqueuing "+
				"Ingress %s/%s for update", vcfg.Namespace,
				vcfg.Name, ing.Namespace, ing.Name)
			worker.queue.Add(&SyncObj{Type: Update, Obj: ing})
		}
	}
	return nil
}

func (worker *NamespaceWorker) syncVcfg(key string) error {
	worker.log.Infof("Syncing VarnishConfig: %s/%s", worker.namespace, key)
	vcfg, err := worker.vcfg.Get(key)
	if err != nil {
		return err
	}
	worker.log.Debugf("VarnishConfig %s/%s: %+v", vcfg.Namespace,
		vcfg.Name, vcfg)

	if len(vcfg.Spec.Services) == 0 {
		// CRD validation should prevent this.
		worker.log.Infof("VarnishConfig %s/%s: no services defined, "+
			"ignoring", vcfg.Namespace, vcfg.Name)
		return nil
	}

	if vcfg.Spec.SelfSharding != nil {
		if err = validateProbe(&vcfg.Spec.SelfSharding.Probe); err != nil {
			return fmt.Errorf("VarnishConfig %s/%s invalid "+
				"sharding spec: %v", vcfg.Namespace, vcfg.Name,
				err)
		}
	}

	return worker.enqueueIngsForVcfg(vcfg)
}

func (worker *NamespaceWorker) addVcfg(key string) error {
	return worker.syncVcfg(key)
}

func (worker *NamespaceWorker) updateVcfg(key string) error {
	return worker.syncVcfg(key)
}

func (worker *NamespaceWorker) deleteVcfg(obj interface{}) error {
	vcfg, ok := obj.(*vcr_v1alpha1.VarnishConfig)
	if !ok || vcfg == nil {
		worker.log.Warnf("Delete VarnishConfig: not found: %v", obj)
		return nil
	}
	worker.log.Infof("Deleting VarnishConfig: %s/%s", vcfg.Namespace,
		vcfg.Name)
	return worker.enqueueIngsForVcfg(vcfg)
}
