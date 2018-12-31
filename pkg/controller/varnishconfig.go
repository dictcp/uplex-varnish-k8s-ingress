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

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
)

// XXX a validation webhook should do this.
// Assume that validation for the CustomResource has already checked
// the Timeout, Interval and Initial fields, and that Window and
// Threshold have been checked for permitted ranges.
func validateSharding(spec *vcr_v1alpha1.SelfShardSpec) error {
	if spec == nil {
		return nil
	}
	if spec.Probe.Window != nil && spec.Probe.Threshold != nil &&
		*spec.Probe.Threshold > *spec.Probe.Window {
		return fmt.Errorf("Threshold (%d) may not be greater than "+
			"Window (%d)", spec.Probe.Threshold, spec.Probe.Window)
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

	if err = validateSharding(vcfg.Spec.SelfSharding); err != nil {
		return fmt.Errorf("VarnishConfig %s/%s invalid sharding "+
			"spec: %v", vcfg.Namespace, vcfg.Name, err)
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
		worker.log.Infof("VarnishConfig %s/%s: enqueuing service %s/%s"+
			" for update", vcfg.Namespace, vcfg.Name,
			svcObj.Namespace, svcObj.Name)
		worker.queue.Add(svcObj)
	}
	return nil
}

func (worker *NamespaceWorker) deleteVcfg(key string) error {
	nsKey := worker.namespace + "/" + key
	worker.log.Info("Deleting VarnishConfig:", nsKey)
	vcfg, err := worker.vcfg.Get(key)
	if err != nil {
		worker.log.Warnf("Cannot get VarnishConfig %s, ignoring: %v",
			nsKey, err)
		return nil
	}
	for _, svc := range vcfg.Spec.Services {
		worker.log.Infof("VarnishConfig %s/%s: enqueuing service %s/%s"+
			" for update", vcfg.Namespace, vcfg.Name,
			worker.namespace, svc)
		worker.queue.Add(svc)
	}
	return nil
}
