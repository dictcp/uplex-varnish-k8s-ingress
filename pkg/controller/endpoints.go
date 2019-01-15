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

import api_v1 "k8s.io/api/core/v1"

func (worker *NamespaceWorker) syncEndp(key string) error {
	worker.log.Infof("Syncing Endpoints: %s/%s", worker.namespace, key)
	svc, err := worker.svc.Get(key)
	if err != nil {
		worker.log.Warnf("Cannot get service for endpoints %s/%s, "+
			"ignoring", worker.namespace, key)
		syncCounters.WithLabelValues(worker.namespace, "Endpoints",
			"Ignore").Inc()
		return nil
	}

	if worker.isVarnishIngSvc(svc) {
		worker.log.Infof("Endpoints changed for Varnish Ingress "+
			"service %s/%s, enqueuing service sync", svc.Namespace,
			svc.Name)
		worker.queue.Add(&SyncObj{Type: Update, Obj: svc})
		return nil
	}

	worker.log.Debugf("Checking ingresses for endpoints: %s/%s",
		worker.namespace, key)
	ings, err := worker.getIngsForSvc(svc)
	if err != nil {
		return err
	}
	if len(ings) == 0 {
		worker.log.Debugf("No ingresses for endpoints: %s/%s",
			worker.namespace, key)
		syncCounters.WithLabelValues(worker.namespace, "Endpoints",
			"Ignore").Inc()
		return nil
	}

	worker.log.Debugf("Update ingresses for endpoints %s", key)
	for _, ing := range ings {
		if !isVarnishIngress(ing) {
			worker.log.Debugf("Ingress %s/%s: not Varnish",
				ing.Namespace, ing.Name)
			continue
		}
		err = worker.addOrUpdateIng(ing)
		if err != nil {
			return err
		}
	}
	return nil
}

func (worker *NamespaceWorker) addEndp(key string) error {
	return worker.syncEndp(key)
}

func (worker *NamespaceWorker) updateEndp(key string) error {
	return worker.syncEndp(key)
}

func (worker *NamespaceWorker) deleteEndp(obj interface{}) error {
	endp, ok := obj.(*api_v1.Endpoints)
	if !ok || endp == nil {
		worker.log.Warnf("Delete Endpoints: not found: %v", obj)
		return nil
	}
	return worker.syncEndp(endp.Name)
}
