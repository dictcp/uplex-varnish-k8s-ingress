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
	api_v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/labels"
)

// XXX make this configurable
const admSecretKey = "admin"

func (worker *NamespaceWorker) getVarnishSvcsForSecret(
	secretName string) ([]*api_v1.Service, error) {

	var secrSvcs []*api_v1.Service
	svcs, err := worker.svc.List(varnishIngressSelector)
	if err != nil {
		return secrSvcs, err
	}
	for _, svc := range svcs {
		pods, err := worker.getPods(svc)
		if err != nil {
			return secrSvcs,
				fmt.Errorf("Error getting pod information for "+
					"service %s/%s: %v", svc.Namespace,
					svc.Name, err)
		}
		if len(pods.Items) == 0 {
			continue
		}

		// The secret is meant for the service if a
		// SecretVolumeSource is specified in the Pod spec
		// that names the secret.
		pod := pods.Items[0]
		for _, vol := range pod.Spec.Volumes {
			if vol.Secret == nil {
				continue
			}
			if vol.Secret.SecretName == secretName {
				secrSvcs = append(secrSvcs, svc)
			}
		}
	}
	return secrSvcs, nil
}

func (worker *NamespaceWorker) updateVcfgsForSecret(secrName string) error {
	var vcfgs []*vcr_v1alpha1.VarnishConfig
	vs, err := worker.vcfg.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, v := range vs {
		for _, auth := range v.Spec.Auth {
			if auth.SecretName == secrName {
				vcfgs = append(vcfgs, v)
			}
		}
	}
	if len(vcfgs) == 0 {
		worker.log.Infof("No VarnishConfigs found for secret: "+
			"%s/%s", worker.namespace, secrName)
		return nil
	}
	for _, vcfg := range vcfgs {
		worker.log.Infof("Requeuing VarnishConfig %s/%s "+
			"after update for secret %s/%s",
			vcfg.Namespace, vcfg.Name, worker.namespace, secrName)
		worker.queue.Add(&SyncObj{Type: Update, Obj: vcfg})
	}
	return nil
}

func (worker *NamespaceWorker) updateVarnishSvcsForSecret(
	svcs []*api_v1.Service, secretKey string) error {

	for _, svc := range svcs {
		svcKey := svc.Namespace + "/" + svc.Name
		if err := worker.vController.
			UpdateSvcForSecret(svcKey, secretKey); err != nil {

			return err
		}
	}
	return nil
}

func (worker *NamespaceWorker) syncSecret(key string) error {
	worker.log.Infof("Syncing Secret: %s/%s", worker.namespace, key)
	secret, err := worker.secr.Get(key)
	if err != nil {
		return err
	}

	app, ok := secret.Labels[labelKey]
	if !ok || app != labelVal {
		worker.log.Infof("Not a Varnish secret: %s/%s",
			secret.Namespace, secret.Name)
		return nil
	}

	svcs, err := worker.getVarnishSvcsForSecret(secret.Name)
	if err != nil {
		return err
	}
	worker.log.Debugf("Found Varnish services for secret %s/%s: %v",
		secret.Namespace, secret.Name, svcs)
	if len(svcs) == 0 {
		worker.log.Infof("No Varnish services with admin secret: %s/%s",
			secret.Namespace, secret.Name)
		return worker.updateVcfgsForSecret(secret.Name)
	}

	secretData, exists := secret.Data[admSecretKey]
	if !exists {
		return fmt.Errorf("Secret %s/%s does not have key %s",
			secret.Namespace, secret.Name, admSecretKey)
	}
	secretKey := secret.Namespace + "/" + secret.Name
	worker.log.Debugf("Setting secret %s", secretKey)
	worker.vController.SetAdmSecret(secretKey, secretData)

	return worker.updateVarnishSvcsForSecret(svcs, secretKey)
}

func (worker *NamespaceWorker) addSecret(key string) error {
	return worker.syncSecret(key)
}

func (worker *NamespaceWorker) updateSecret(key string) error {
	return worker.syncSecret(key)
}

func (worker *NamespaceWorker) deleteSecret(obj interface{}) error {
	secr, ok := obj.(*api_v1.Secret)
	if !ok || secr == nil {
		worker.log.Warnf("Delete Secret: not found: %v", obj)
		return nil
	}
	worker.log.Infof("Deleting Secret: %s/%s", secr.Namespace, secr.Name)
	svcs, err := worker.getVarnishSvcsForSecret(secr.Name)
	if err != nil {
		return err
	}
	worker.log.Debugf("Found Varnish services for secret %s/%s: %v",
		secr.Namespace, secr.Name, svcs)
	if len(svcs) == 0 {
		worker.log.Infof("No Varnish services with admin secret: %s/%s",
			secr.Namespace, secr.Name)
		return worker.updateVcfgsForSecret(secr.Name)
	}

	secretKey := secr.Namespace + "/" + secr.Name
	worker.vController.DeleteAdmSecret(secretKey)

	return worker.updateVarnishSvcsForSecret(svcs, secretKey)
}
