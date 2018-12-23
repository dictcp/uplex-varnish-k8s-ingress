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

// Methods for syncing Ingresses

import (
	"fmt"
	"strings"

	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/varnish/vcl"

	"k8s.io/apimachinery/pkg/util/intstr"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

const (
	ingressClassKey        = "kubernetes.io/ingress.class"
	annotationPrefix       = "ingress.varnish-cache.org/"
	selfShardKey           = "self-sharding"
	shardProbeTimeoutKey   = "self-sharding-probe-timeout"
	shardProbeIntervalKey  = "self-sharding-probe-interval"
	shardProbeInitialKey   = "self-sharding-probe-initial"
	shardProbeWindowKey    = "self-sharding-probe-window"
	shardProbeThresholdKey = "self-sharding-probe-threshold"
	shardMax2ndTTL         = "self-sharding-max-secondary-ttl"
	varnishSvcKey          = annotationPrefix + "varnish-svc"
)

// XXX an annotation to identify the Service for an Ingress
func (worker *NamespaceWorker) getVarnishSvcForIng(
	ing *extensions.Ingress) (*api_v1.Service, error) {

	svcs, err := worker.svc.List(varnishIngressSelector)
	if err != nil {
		return nil, err
	}
	if varnishSvc, exists := ing.Annotations[varnishSvcKey]; exists {
		for _, svc := range svcs {
			if svc.Name == varnishSvc {
				return svc, nil
			}
		}
		return nil, nil
	}
	if len(svcs) == 1 {
		return svcs[0], nil
	}
	return nil, nil
}

func (worker *NamespaceWorker) ingBackend2Addrs(
	backend extensions.IngressBackend) (addrs []vcl.Address, err error) {

	svc, err := worker.svc.Get(backend.ServiceName)
	if err != nil {
		return
	}

	endps, err := worker.getServiceEndpoints(svc)
	if err != nil {
		return addrs, fmt.Errorf("Error getting endpoints for service "+
			"%v: %v", svc, err)
	}

	targetPort := int32(0)
	ingSvcPort := backend.ServicePort
	for _, port := range svc.Spec.Ports {
		if (ingSvcPort.Type == intstr.Int &&
			port.Port == int32(ingSvcPort.IntValue())) ||
			(ingSvcPort.Type == intstr.String &&
				port.Name == ingSvcPort.String()) {

			targetPort, err = worker.getTargetPort(&port, svc)
			if err != nil {
				return addrs, fmt.Errorf("Error determining "+
					"target port for port %v in Ingress: "+
					"%v", ingSvcPort, err)
			}
			break
		}
	}
	if targetPort == 0 {
		return addrs, fmt.Errorf("No port %v in service %s/%s",
			ingSvcPort, svc.Namespace, svc.Name)
	}

	return endpsTargetPort2Addrs(svc, endps, targetPort)
}

func (worker *NamespaceWorker) ing2VCLSpec(
	ing *extensions.Ingress) (vcl.Spec, error) {

	vclSpec := vcl.Spec{}
	vclSpec.AllServices = make(map[string]vcl.Service)
	if ing.Spec.TLS != nil && len(ing.Spec.TLS) > 0 {
		worker.log.Warnf("TLS config currently ignored in Ingress %s",
			ing.ObjectMeta.Name)
	}
	if ing.Spec.Backend != nil {
		backend := ing.Spec.Backend
		addrs, err := worker.ingBackend2Addrs(*backend)
		if err != nil {
			return vclSpec, err
		}
		vclSvc := vcl.Service{
			Name:      backend.ServiceName,
			Addresses: addrs,
		}
		vclSpec.DefaultService = vclSvc
		vclSpec.AllServices[backend.ServiceName] = vclSvc
	}
	for _, rule := range ing.Spec.Rules {
		if rule.Host == "" {
			return vclSpec, fmt.Errorf("Ingress rule contains " +
				"empty Host")
		}
		vclRule := vcl.Rule{Host: rule.Host}
		vclRule.PathMap = make(map[string]vcl.Service)
		if rule.IngressRuleValue.HTTP == nil {
			vclSpec.Rules = append(vclSpec.Rules, vclRule)
			continue
		}
		for _, path := range rule.IngressRuleValue.HTTP.Paths {
			addrs, err := worker.ingBackend2Addrs(path.Backend)
			if err != nil {
				return vclSpec, err
			}
			vclSvc := vcl.Service{
				Name:      path.Backend.ServiceName,
				Addresses: addrs,
			}
			vclRule.PathMap[path.Path] = vclSvc
			vclSpec.AllServices[path.Backend.ServiceName] = vclSvc
		}
		vclSpec.Rules = append(vclSpec.Rules, vclRule)
	}
	return vclSpec, nil
}

func (worker *NamespaceWorker) configSharding(spec *vcl.Spec,
	ing *extensions.Ingress, svc *api_v1.Service) error {

	ann, exists := ing.Annotations[annotationPrefix+selfShardKey]
	if !exists ||
		(!strings.EqualFold(ann, "on") &&
			!strings.EqualFold(ann, "true")) {
		worker.log.Debugf("No cluster shard configuration for Ingress "+
			"%s/%s", ing.Namespace, ing.Name)
		return nil
	}

	worker.log.Debugf("Set cluster shard configuration for Ingress %s/%s",
		ing.Namespace, ing.Name)

	pods, err := worker.getPods(svc)
	if err != nil {
		return fmt.Errorf("Error getting pod information for service "+
			"%s/%s: %v", svc.Namespace, svc.Name, err)
	}
	if len(pods.Items) <= 1 {
		return fmt.Errorf("Sharding requested, but %d pods found for "+
			"service %s/%s", len(pods.Items), svc.Namespace,
			svc.Name)
	}

	worker.log.Debugf("Pods for shard configuration: %+v", pods.Items)

	// Populate spec.ShardCluster.Nodes with Pod names and the http endpoint
	for _, pod := range pods.Items {
		var varnishCntnr api_v1.Container
		var httpPort int32
		for _, c := range pod.Spec.Containers {
			if c.Image == "varnish-ingress/varnish" {
				varnishCntnr = c
				break
			}
		}
		if varnishCntnr.Image != "varnish-ingress/varnish" {
			return fmt.Errorf("No Varnish container found in Pod "+
				"%s for service %s/%s", pod.Name, svc.Namespace,
				svc.Name)
		}
		for _, p := range varnishCntnr.Ports {
			if p.Name == "http" {
				httpPort = p.ContainerPort
				break
			}
		}
		if httpPort == 0 {
			return fmt.Errorf("No http port found in Pod %s for "+
				"service %s/%s", pod.Name, svc.Namespace,
				svc.Name)
		}
		node := vcl.Service{Addresses: make([]vcl.Address, 1)}
		if pod.Spec.Hostname != "" {
			node.Name = pod.Spec.Hostname
		} else {
			node.Name = pod.Name
		}
		node.Addresses[0].IP = pod.Status.PodIP
		node.Addresses[0].Port = httpPort
		spec.ShardCluster.Nodes = append(spec.ShardCluster.Nodes, node)
	}
	worker.log.Debugf("Node configuration for self-sharding in Ingress "+
		"%s/%s: %+v", ing.Namespace, ing.Name, spec.ShardCluster.Nodes)

	anns := ing.Annotations
	ann, exists = anns[annotationPrefix+shardProbeTimeoutKey]
	if exists {
		spec.ShardCluster.Probe.Timeout = ann
	}
	ann, exists = anns[annotationPrefix+shardProbeIntervalKey]
	if exists {
		spec.ShardCluster.Probe.Interval = ann
	}
	ann, exists = anns[annotationPrefix+shardProbeInitialKey]
	if exists {
		spec.ShardCluster.Probe.Initial = ann
	}
	ann, exists = anns[annotationPrefix+shardProbeWindowKey]
	if exists {
		spec.ShardCluster.Probe.Window = ann
	}
	ann, exists = anns[annotationPrefix+shardProbeThresholdKey]
	if exists {
		spec.ShardCluster.Probe.Threshold = ann
	}
	ann, exists = anns[annotationPrefix+shardMax2ndTTL]
	if exists {
		spec.ShardCluster.MaxSecondaryTTL = ann
	}
	worker.log.Debugf("Spec configuration for self-sharding in Ingress "+
		"%s/%s: %+v", ing.Namespace, ing.Name, spec.ShardCluster)
	return nil
}

func (worker *NamespaceWorker) hasIngress(svc *api_v1.Service,
	ing *extensions.Ingress, spec vcl.Spec) bool {

	svcKey := svc.Namespace + "/" + svc.Name
	ingKey := ing.Namespace + "/" + ing.Name
	return worker.vController.HasIngress(svcKey, ingKey, string(ing.UID),
		spec)
}

func (worker *NamespaceWorker) addOrUpdateIng(ing *extensions.Ingress) error {
	ingKey := ing.ObjectMeta.Namespace + "/" + ing.ObjectMeta.Name
	worker.log.Infof("Adding or Updating Ingress: %s", ingKey)

	// Get the Varnish Service and its Pods
	svc, err := worker.getVarnishSvcForIng(ing)
	if err != nil {
		return err
	}
	if svc == nil {
		return fmt.Errorf("No Varnish Service found for Ingress %s/%s",
			ing.Namespace, ing.Name)
	}
	svcKey := svc.Namespace + "/" + svc.Name
	worker.log.Infof("Ingress %s to be implemented by Varnish Service %s",
		ingKey, svcKey)

	vclSpec, err := worker.ing2VCLSpec(ing)
	if err != nil {
		return err
	}

	if err = worker.configSharding(&vclSpec, ing, svc); err != nil {
		return err
	}

	worker.log.Debugf("Check if Ingress is loaded: key=%s uuid=%s hash=%0x",
		ingKey, string(ing.UID), vclSpec.Canonical().DeepHash())
	if worker.hasIngress(svc, ing, vclSpec) {
		worker.log.Infof("Ingress %s uid=%s hash=%0x already loaded",
			ingKey, ing.UID, vclSpec.Canonical().DeepHash())
		return nil
	}

	worker.log.Debugf("Update Ingress key=%s svc=%s uuid=%s: %+v", ingKey,
		svcKey, string(ing.ObjectMeta.UID), vclSpec)
	err = worker.vController.Update(svcKey, ingKey,
		string(ing.ObjectMeta.UID), vclSpec)
	if err != nil {
		return err
	} else {
		worker.log.Debugf("Updated Ingress key=%s uuid=%s svc=%s: %+v",
			ingKey, string(ing.ObjectMeta.UID), svcKey, vclSpec)
	}
	return nil
}

// We only handle Ingresses with the class annotation "varnish".
func isVarnishIngress(ing *extensions.Ingress) bool {
	class, exists := ing.Annotations[ingressClassKey]
	return exists && class == "varnish"
}

func (worker *NamespaceWorker) syncIng(key string) error {
	nsKey := worker.namespace + "/" + key
	worker.log.Info("Syncing Ingress:", nsKey)
	ing, err := worker.ing.Get(key)
	if err != nil {
		return err
	}

	if !isVarnishIngress(ing) {
		worker.log.Infof("Ignoring Ingress %s/%s, Annotation '%v' "+
			"absent or is not 'varnish'", ing.Namespace, ing.Name,
			ingressClassKey)
		return nil
	}
	return worker.addOrUpdateIng(ing)
}

func (worker *NamespaceWorker) deleteIng(key string) error {
	ing, err := worker.ing.Get(key)
	if err != nil || ing == nil {
		// XXX should clean up Varnish config nevertheless
		worker.log.Warnf("Delete Ingress %s: not found (%v)", key, err)
		return nil
	}
	svc, err := worker.getVarnishSvcForIng(ing)
	if err != nil {
		return err
	}
	if svc == nil {
		return fmt.Errorf("No Varnish Service found for Ingress %s/%s",
			ing.Namespace, ing.Name)
	}
	ingKey := ing.Namespace + "/" + ing.Name
	svcKey := svc.Namespace + "/" + svc.Name
	worker.log.Infof("Deleting Ingress %s (Varnish service %s):", ingKey,
		svcKey)
	return worker.vController.DeleteIngress(svcKey, ingKey)
}
