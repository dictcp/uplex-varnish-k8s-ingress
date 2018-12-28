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

	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

// XXX make this configurable
const admPortName = "varnishadm"

// isVarnishIngSvc determines if a Service represents a Varnish that
// can implement Ingress, for which this controller is responsible.
// Currently the app label must point to a hardwired name.
func (worker *NamespaceWorker) isVarnishIngSvc(svc *api_v1.Service) bool {
	app, exists := svc.Labels[labelKey]
	return exists && app == labelVal
}

func (worker *NamespaceWorker) getIngsForSvc(
	svc *api_v1.Service) (ings []*extensions.Ingress, err error) {

	allIngs, err := worker.ing.List(labels.Everything())
	if err != nil {
		return
	}

	for _, ing := range allIngs {
		if ing.Namespace != svc.Namespace {
			// Shouldn't be possible
			continue
		}
		cpy := ing.DeepCopy()
		if cpy.Spec.Backend != nil {
			if cpy.Spec.Backend.ServiceName == svc.Name {
				ings = append(ings, cpy)
			}
		}
		for _, rules := range cpy.Spec.Rules {
			if rules.IngressRuleValue.HTTP == nil {
				continue
			}
			for _, p := range rules.IngressRuleValue.HTTP.Paths {
				if p.Backend.ServiceName == svc.Name {
					ings = append(ings, cpy)
				}
			}
		}
	}

	if len(ings) == 0 {
		worker.log.Infof("No Varnish Ingresses defined for service %s/%s",
			svc.Namespace, svc.Name)
	}
	return ings, nil
}

func (worker *NamespaceWorker) enqueueIngressForService(
	svc *api_v1.Service) error {

	ings, err := worker.getIngsForSvc(svc)
	if err != nil {
		return err
	}
	for _, ing := range ings {
		if !isVarnishIngress(ing) {
			continue
		}
		worker.queue.Add(ing)
	}
	return nil
}

// Return true if changes in Varnish services may lead to changes in
// the VCL config generated for the Ingress.
func (worker *NamespaceWorker) isVarnishInVCLSpec(ing *extensions.Ingress) bool {
	vcfgs, err := worker.vcfg.List(labels.Everything())
	if err != nil {
		worker.log.Warnf("Error retrieving VarnishConfigs in "+
			"namespace %s: %v", worker.namespace, err)
		return false
	}
	for _, vcfg := range vcfgs {
		if vcfg.Spec.SelfSharding != nil {
			return true
		}
	}
	return false
}

func (worker *NamespaceWorker) syncSvc(key string) error {
	var addrs []vcl.Address
	worker.log.Infof("Syncing Service: %s/%s", worker.namespace, key)
	svc, err := worker.svc.Get(key)
	if err != nil {
		return err
	}
	if !worker.isVarnishIngSvc(svc) {
		return worker.enqueueIngressForService(svc)
	}

	worker.log.Infof("Syncing Varnish Ingress Service %s/%s:",
		svc.Namespace, svc.Name)

	// Check if there are Ingresses for which the VCL spec may
	// change due to changes in Varnish services.
	updateVCL := false
	ings, err := worker.ing.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, ing := range ings {
		if ing.Namespace != svc.Namespace {
			continue
		}
		ingSvc, err := worker.getVarnishSvcForIng(ing)
		if err != nil {
			return err
		}
		if ingSvc.Namespace != svc.Namespace ||
			ingSvc.Name != svc.Name {
			continue
		}
		if !worker.isVarnishInVCLSpec(ing) {
			continue
		}
		updateVCL = true
		worker.log.Debugf("Requeueing Ingress %s/%s after changed "+
			"Varnish service %s/%s: %+v", ing.Namespace,
			ing.Name, svc.Namespace, svc.Name, ing)
		worker.queue.Add(ing)
	}
	if !updateVCL {
		worker.log.Debugf("No change in VCL due to changed Varnish "+
			"service %s/%s", svc.Namespace, svc.Name)
	}

	endps, err := worker.getServiceEndpoints(svc)
	if err != nil {
		return err
	}
	worker.log.Debugf("Varnish service %s/%s endpoints: %+v", svc.Namespace,
		svc.Name, endps)

	// Get the secret name and admin port for the service. We have
	// to retrieve a Pod spec for the service, then look for the
	// SecretVolumeSource, and the port matching admPortName.
	secrName := ""
	worker.log.Debugf("Searching Pods for the secret for %s/%s",
		svc.Namespace, svc.Name)
	pods, err := worker.getPods(svc)
	if err != nil {
		return fmt.Errorf("Cannot get a Pod for service %s/%s: %v",
			svc.Namespace, svc.Name, err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("No Pods for Service: %s/%s", svc.Namespace,
			svc.Name)
	}
	pod := &pods.Items[0]
	for _, vol := range pod.Spec.Volumes {
		if secretVol := vol.Secret; secretVol != nil {
			secrName = secretVol.SecretName
			break
		}
	}
	if secrName != "" {
		secrName = worker.namespace + "/" + secrName
		worker.log.Infof("Found secret name %s for Service %s/%s",
			secrName, svc.Namespace, svc.Name)
	} else {
		worker.log.Warnf("No secret found for Service %s/%s",
			svc.Namespace, svc.Name)
	}

	// XXX hard-wired Port name
	for _, subset := range endps.Subsets {
		admPort := int32(0)
		for _, port := range subset.Ports {
			if port.Name == admPortName {
				admPort = port.Port
				break
			}
		}
		if admPort == 0 {
			return fmt.Errorf("No Varnish admin port %s found for "+
				"Service %s/%s endpoint", admPortName,
				svc.Namespace, svc.Name)
		}
		for _, address := range subset.Addresses {
			addr := vcl.Address{
				IP:   address.IP,
				Port: admPort,
			}
			addrs = append(addrs, addr)
		}
	}
	worker.log.Debugf("Varnish service %s/%s addresses: %+v", svc.Namespace,
		svc.Name, addrs)
	return worker.vController.AddOrUpdateVarnishSvc(
		svc.Namespace+"/"+svc.Name, addrs, secrName, !updateVCL)
}

func (worker *NamespaceWorker) deleteSvc(key string) error {
	nsKey := worker.namespace + "/" + key
	worker.log.Info("Deleting Service:", nsKey)
	svcObj, err := worker.svc.Get(key)
	if err != nil {
		return err
	}
	if !worker.isVarnishIngSvc(svcObj) {
		return worker.enqueueIngressForService(svcObj)
	}

	return worker.vController.DeleteVarnishSvc(nsKey)
}
