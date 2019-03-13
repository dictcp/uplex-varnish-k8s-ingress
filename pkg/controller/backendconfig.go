/*
 * Copyright (c) 2019 UPLEX Nils Goroll Systemoptimierung
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

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	extensions "k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

func (worker *NamespaceWorker) enqueueIngsForBackendSvcs(svcs []string,
	namespace, name string) error {

	svc2ing := make(map[string]*extensions.Ingress)
	ings, err := worker.ing.List(labels.Everything())
	if errors.IsNotFound(err) {
		worker.log.Infof("BackendConfig %s/%s: no Ingresses found in "+
			"workspace %s", namespace, name, worker.namespace)
		return nil
	}
	if err != nil {
		return err
	}
	for _, ing := range ings {
		if ing.Spec.Backend != nil {
			svc2ing[ing.Spec.Backend.ServiceName] = ing
		}
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				svc2ing[path.Backend.ServiceName] = ing
			}
		}
	}

	svcSet := make(map[string]struct{})
	for _, svc := range svcs {
		if _, exists := svcSet[svc]; exists {
			continue
		}
		svcSet[svc] = struct{}{}

		if ing, exists := svc2ing[svc]; exists {
			worker.log.Infof("BackendConfig %s/%s: enqueuing "+
				"Ingress %s/%s for update", namespace, name,
				ing.Namespace, ing.Name)
			worker.queue.Add(&SyncObj{Type: Update, Obj: ing})
		}
	}
	return nil
}

func (worker *NamespaceWorker) syncBcfg(key string) error {
	worker.log.Infof("Syncing BackendConfig: %s/%s", worker.namespace, key)
	bcfg, err := worker.bcfg.Get(key)
	if err != nil {
		return err
	}
	worker.log.Tracef("BackendConfig %s/%s: %+v", bcfg.Namespace,
		bcfg.Name, bcfg)

	if len(bcfg.Spec.Services) == 0 {
		// CRD validation should prevent this.
		worker.log.Warnf("BackendConfig %s/%s: no services defined, "+
			"ignoring", bcfg.Namespace, bcfg.Name)
		syncCounters.WithLabelValues(worker.namespace, "BackendConfig",
			"Ignore").Inc()
		return nil
	}

	if err = validateProbe(bcfg.Spec.Probe); err != nil {
		return fmt.Errorf("BackendConfig %s/%s invalid probe "+
			"spec: %v", bcfg.Namespace, bcfg.Name, err)
	}

	worker.log.Infof("BackendConfig %s/%s: enqueue Ingresses for "+
		"Services: %+v", bcfg.Namespace, bcfg.Name, bcfg.Spec.Services)
	return worker.enqueueIngsForBackendSvcs(bcfg.Spec.Services,
		bcfg.Namespace, bcfg.Name)
}

func (worker *NamespaceWorker) addBcfg(key string) error {
	return worker.syncBcfg(key)
}

func (worker *NamespaceWorker) updateBcfg(key string) error {
	return worker.syncBcfg(key)
}

func (worker *NamespaceWorker) deleteBcfg(obj interface{}) error {
	bcfg, ok := obj.(*vcr_v1alpha1.BackendConfig)
	if !ok || bcfg == nil {
		worker.log.Warnf("Delete BackendConfig: not found: %v", obj)
		return nil
	}
	worker.log.Infof("Deleting BackendConfig: %s/%s", bcfg.Namespace,
		bcfg.Name)
	return worker.enqueueIngsForBackendSvcs(bcfg.Spec.Services,
		bcfg.Namespace, bcfg.Name)
}
