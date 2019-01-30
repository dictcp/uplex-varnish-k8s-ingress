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
	"encoding/base64"
	"fmt"
	"strconv"

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

const (
	ingressClassKey  = "kubernetes.io/ingress.class"
	annotationPrefix = "ingress.varnish-cache.org/"
	varnishSvcKey    = annotationPrefix + "varnish-svc"
	defACLcomparand  = "client.ip"
	defACLfailStatus = uint16(403)
)

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

func getVCLProbe(probe *vcr_v1alpha1.ProbeSpec) *vcl.Probe {
	if probe == nil {
		return nil
	}
	vclProbe := &vcl.Probe{
		URL:      probe.URL,
		Request:  make([]string, len(probe.Request)),
		Timeout:  probe.Timeout,
		Interval: probe.Interval,
	}
	for i, r := range probe.Request {
		vclProbe.Request[i] = r
	}
	if probe.ExpResponse != nil {
		vclProbe.ExpResponse = uint16(*probe.ExpResponse)
	}
	if probe.Initial != nil {
		vclProbe.Initial = strconv.Itoa((int(*probe.Window)))
	}
	if probe.Window != nil {
		vclProbe.Window = strconv.Itoa((int(*probe.Window)))
	}
	if probe.Threshold != nil {
		vclProbe.Threshold = strconv.Itoa((int(*probe.Threshold)))
	}
	return vclProbe
}

func (worker *NamespaceWorker) getVCLSvc(svcName string,
	addrs []vcl.Address) (vcl.Service, error) {

	vclSvc := vcl.Service{
		Name:      svcName,
		Addresses: addrs,
	}
	bcfgs, err := worker.bcfg.List(labels.Everything())
	if err != nil {
		if errors.IsNotFound(err) {
			return vclSvc, nil
		}
		return vclSvc, err
	}
	var bcfg *vcr_v1alpha1.BackendConfig
BCfgs:
	for _, b := range bcfgs {
		// XXX report error if > 1 BackendConfig for the Service
		for _, svc := range b.Spec.Services {
			if svc == svcName {
				bcfg = b
				break BCfgs
			}
		}
	}
	if bcfg == nil {
		return vclSvc, nil
	}
	if bcfg.Spec.Director != nil {
		vclSvc.Director = &vcl.Director{
			Type: vcl.GetDirectorType(
				string(bcfg.Spec.Director.Type)),
			Rampup: bcfg.Spec.Director.Rampup,
		}
		if bcfg.Spec.Director.Warmup != nil {
			vclSvc.Director.Warmup =
				float64(*bcfg.Spec.Director.Warmup) / 100.0
		}
	}
	// XXX if bcfg.Spec.Probe == nil, look for a HTTP readiness check
	// in the ContainerSpec.
	vclSvc.Probe = getVCLProbe(bcfg.Spec.Probe)
	vclSvc.HostHeader = bcfg.Spec.HostHeader
	vclSvc.ConnectTimeout = bcfg.Spec.ConnectTimeout
	vclSvc.FirstByteTimeout = bcfg.Spec.FirstByteTimeout
	vclSvc.BetweenBytesTimeout = bcfg.Spec.BetweenBytesTimeout
	if bcfg.Spec.MaxConnections != nil {
		vclSvc.MaxConnections = uint32(*bcfg.Spec.MaxConnections)
	}
	if bcfg.Spec.ProxyHeader != nil {
		vclSvc.ProxyHeader = uint8(*bcfg.Spec.ProxyHeader)
	}
	return vclSvc, nil
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
		vclSvc, err := worker.getVCLSvc(backend.ServiceName, addrs)
		if err != nil {
			return vclSpec, err
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
			vclSvc, err := worker.getVCLSvc(
				path.Backend.ServiceName, addrs)
			if err != nil {
				return vclSpec, nil
			}
			vclRule.PathMap[path.Path] = vclSvc
			vclSpec.AllServices[path.Backend.ServiceName] = vclSvc
		}
		vclSpec.Rules = append(vclSpec.Rules, vclRule)
	}
	return vclSpec, nil
}

func (worker *NamespaceWorker) configSharding(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig, svc *api_v1.Service) error {

	if vcfg.Spec.SelfSharding == nil {
		worker.log.Debugf("No cluster shard configuration for Service "+
			"%s/%s", svc.Namespace, svc.Name)
		return nil
	}

	worker.log.Debugf("Set cluster shard configuration for Service %s/%s",
		svc.Namespace, svc.Name)

	endps, err := worker.getServiceEndpoints(svc)
	if err != nil {
		return fmt.Errorf("Error getting endpoints for service %s/%s: "+
			"%v", svc.Namespace, svc.Name, err)
	}
	worker.log.Debugf("Endpoints for shard configuration: %+v", endps)

	var nAddrs int
	var httpPort int32
	for _, endpSubset := range endps.Subsets {
		nAddrs += len(endpSubset.Addresses)
		nAddrs += len(endpSubset.NotReadyAddresses)
		for _, port := range endpSubset.Ports {
			if httpPort == 0 && port.Name == "http" {
				httpPort = port.Port
			}
		}
	}
	if httpPort == 0 {
		return fmt.Errorf("No http port found in the endpoints for "+
			"service %s/%s", svc.Namespace, svc.Name)
	}
	if nAddrs <= 1 {
		return fmt.Errorf("Sharding requested, but %d endpoint "+
			"addresses found for service %s/%s", nAddrs,
			svc.Namespace, svc.Name)
	}

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
	worker.log.Debugf("Node configuration for self-sharding in Service "+
		"%s/%s: %+v", svc.Namespace, svc.Name, spec.ShardCluster.Nodes)

	cfgSpec := vcfg.Spec.SelfSharding
	probe := getVCLProbe(&cfgSpec.Probe)
	spec.ShardCluster.Probe = *probe
	if cfgSpec.Max2ndTTL != "" {
		spec.ShardCluster.MaxSecondaryTTL = cfgSpec.Max2ndTTL
	}
	worker.log.Debugf("Spec configuration for self-sharding in Service "+
		"%s/%s: %+v", svc.Namespace, svc.Name, spec.ShardCluster)
	return nil
}

func (worker *NamespaceWorker) configAuth(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	if len(vcfg.Spec.Auth) == 0 {
		worker.log.Infof("No Auth spec found for VarnishConfig %s/%s",
			vcfg.Namespace, vcfg.Name)
		return nil
	}
	worker.log.Debugf("VarnishConfig %s/%s: configure %d VCL auths",
		vcfg.Namespace, vcfg.Name, len(vcfg.Spec.Auth))
	spec.Auths = make([]vcl.Auth, 0, len(vcfg.Spec.Auth))
	for _, auth := range vcfg.Spec.Auth {
		worker.log.Debugf("VarnishConfig %s/%s configuring VCL auth "+
			"from: %+v", vcfg.Namespace, vcfg.Name, auth)
		secret, err := worker.secr.Get(auth.SecretName)
		if err != nil {
			return err
		}
		if len(secret.Data) == 0 {
			worker.log.Warnf("No secrets found in Secret %s/%s "+
				"for realm %s in VarnishConfig %s/%s, ignoring",
				secret.Namespace, secret.Name, auth.Realm,
				vcfg.Namespace, vcfg.Name)
			continue
		}
		worker.log.Debugf("VarnishConfig %s/%s configure %d "+
			"credentials for realm %s", vcfg.Namespace, vcfg.Name,
			len(secret.Data), auth.Realm)
		vclAuth := vcl.Auth{
			Realm:       auth.Realm,
			Credentials: make([]string, 0, len(secret.Data)),
			UTF8:        auth.UTF8,
		}
		if auth.Type == "" || auth.Type == vcr_v1alpha1.Basic {
			vclAuth.Status = vcl.Basic
		} else {
			vclAuth.Status = vcl.Proxy
		}
		for user, pass := range secret.Data {
			str := user + ":" + string(pass)
			cred := base64.StdEncoding.EncodeToString([]byte(str))
			worker.log.Debugf("VarnishConfig %s/%s: add cred %s "+
				"for realm %s to VCL config", vcfg.Namespace,
				vcfg.Name, cred, vclAuth.Realm)
			vclAuth.Credentials = append(vclAuth.Credentials, cred)
		}
		if auth.Condition != nil {
			vclAuth.Condition.URLRegex = auth.Condition.URLRegex
			vclAuth.Condition.HostRegex = auth.Condition.HostRegex
		}
		worker.log.Debugf("VarnishConfig %s/%s add VCL auth config: "+
			"%+v", vcfg.Namespace, vcfg.Name, vclAuth)
		spec.Auths = append(spec.Auths, vclAuth)
	}
	return nil
}

func (worker *NamespaceWorker) configACL(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	if len(vcfg.Spec.ACLs) == 0 {
		worker.log.Infof("No ACL spec found for VarnishConfig %s/%s",
			vcfg.Namespace, vcfg.Name)
		return nil
	}
	spec.ACLs = make([]vcl.ACL, len(vcfg.Spec.ACLs))
	for i, acl := range vcfg.Spec.ACLs {
		worker.log.Infof("VarnishConfig %s/%s configuring ACL %s",
			vcfg.Namespace, vcfg.Name, acl.Name)
		worker.log.Debugf("ACL %s: %+v", acl.Name, acl)
		vclACL := vcl.ACL{
			Name:       acl.Name,
			Addresses:  make([]vcl.ACLAddress, len(acl.Addresses)),
			Conditions: make([]vcl.MatchTerm, len(acl.Conditions)),
		}
		if acl.Comparand == "" {
			vclACL.Comparand = defACLcomparand
		} else {
			vclACL.Comparand = acl.Comparand
		}
		if acl.ACLType == "" || acl.ACLType == vcr_v1alpha1.Whitelist {
			vclACL.Whitelist = true
		}
		if acl.FailStatus == nil {
			vclACL.FailStatus = defACLfailStatus
		} else {
			vclACL.FailStatus = uint16(*acl.FailStatus)
		}
		for j, addr := range acl.Addresses {
			vclAddr := vcl.ACLAddress{
				Addr:   addr.Address,
				Negate: addr.Negate,
			}
			if addr.MaskBits == nil {
				vclAddr.MaskBits = vcl.NoMaskBits
			} else {
				vclAddr.MaskBits = uint8(*addr.MaskBits)
			}
			vclACL.Addresses[j] = vclAddr
		}
		for j, cond := range acl.Conditions {
			vclMatch := vcl.MatchTerm{
				Comparand: cond.Comparand,
				Value:     cond.Value,
			}
			switch cond.Compare {
			case vcr_v1alpha1.Equal:
				vclMatch.Compare = vcl.Equal
			case vcr_v1alpha1.NotEqual:
				vclMatch.Compare = vcl.NotEqual
			case vcr_v1alpha1.Match:
				vclMatch.Compare = vcl.Match
			case vcr_v1alpha1.NotMatch:
				vclMatch.Compare = vcl.NotMatch
			}
			vclACL.Conditions[j] = vclMatch
		}
		worker.log.Debugf("VarnishConfig %s/%s add VCL ACL config: "+
			"%+v", vcfg.Namespace, vcfg.Name, vclACL)
		spec.ACLs[i] = vclACL
	}
	return nil
}

func (worker *NamespaceWorker) configRewrites(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	if len(vcfg.Spec.Rewrites) == 0 {
		worker.log.Infof("No rewrite spec found for VarnishConfig "+
			"%s/%s", vcfg.Namespace, vcfg.Name)
		return nil
	}
	spec.Rewrites = make([]vcl.Rewrite, len(vcfg.Spec.Rewrites))
	for i, rw := range vcfg.Spec.Rewrites {
		worker.log.Infof("VarnishConfig %s/%s: configuring rewrite "+
			"for target %s", vcfg.Namespace, vcfg.Name, rw.Target)
		worker.log.Debugf("Rewrite: %v", rw)
		vclRw := vcl.Rewrite{
			Target: rw.Target,
			Rules:  make([]vcl.RewriteRule, len(rw.Rules)),
		}
		for j := range rw.Rules {
			vclRw.Rules[j] = vcl.RewriteRule{
				Value:   rw.Rules[j].Value,
				Rewrite: rw.Rules[j].Rewrite,
			}
		}
		if rw.Source == "" {
			// XXX
			// The Source is the same as the Target if:
			// - Method is one of sub, suball or rewrite,
			// - or Method is one of replace, append or
			//   prepend, and there are either no rules
			//   or more than one rule.
			if rw.Method == vcr_v1alpha1.Sub ||
				rw.Method == vcr_v1alpha1.Suball ||
				rw.Method == vcr_v1alpha1.Rewrite ||
				((rw.Method == vcr_v1alpha1.Replace ||
					rw.Method == vcr_v1alpha1.Append ||
					rw.Method == vcr_v1alpha1.Prepend) &&
					len(rw.Rules) != 1) {

				vclRw.Source = rw.Target
			}
		} else {
			vclRw.Source = rw.Source
		}

		switch rw.Method {
		case vcr_v1alpha1.Replace:
			vclRw.Method = vcl.Replace
		case vcr_v1alpha1.Sub:
			vclRw.Method = vcl.Sub
		case vcr_v1alpha1.Suball:
			vclRw.Method = vcl.Suball
		case vcr_v1alpha1.Rewrite:
			vclRw.Method = vcl.RewriteMethod
		case vcr_v1alpha1.Append:
			vclRw.Method = vcl.Append
		case vcr_v1alpha1.Prepend:
			vclRw.Method = vcl.Prepend
		case vcr_v1alpha1.Delete:
			vclRw.Method = vcl.Delete
		default:
			return fmt.Errorf("Illegal method %s", rw.Method)
		}

		switch rw.Compare {
		case vcr_v1alpha1.RewriteMatch:
			vclRw.Compare = vcl.RewriteMatch
		case vcr_v1alpha1.RewriteEqual:
			vclRw.Compare = vcl.RewriteEqual
		case vcr_v1alpha1.Prefix:
			vclRw.Compare = vcl.Prefix
		default:
			vclRw.Compare = vcl.RewriteMatch
		}

		switch rw.VCLSub {
		case vcr_v1alpha1.Recv:
			vclRw.VCLSub = vcl.Recv
		case vcr_v1alpha1.Pipe:
			vclRw.VCLSub = vcl.Pipe
		case vcr_v1alpha1.Pass:
			vclRw.VCLSub = vcl.Pass
		case vcr_v1alpha1.Hash:
			vclRw.VCLSub = vcl.Hash
		case vcr_v1alpha1.Purge:
			vclRw.VCLSub = vcl.Purge
		case vcr_v1alpha1.Miss:
			vclRw.VCLSub = vcl.Miss
		case vcr_v1alpha1.Hit:
			vclRw.VCLSub = vcl.Hit
		case vcr_v1alpha1.Deliver:
			vclRw.VCLSub = vcl.Deliver
		case vcr_v1alpha1.Synth:
			vclRw.VCLSub = vcl.Synth
		case vcr_v1alpha1.BackendFetch:
			vclRw.VCLSub = vcl.BackendFetch
		case vcr_v1alpha1.BackendResponse:
			vclRw.VCLSub = vcl.BackendResponse
		case vcr_v1alpha1.BackendError:
			vclRw.VCLSub = vcl.BackendError
		default:
			vclRw.VCLSub = vcl.Unspecified
		}

		if rw.MatchFlags != nil {
			vclRw.MatchFlags = vcl.MatchFlagsType{
				UTF8:         rw.MatchFlags.UTF8,
				PosixSyntax:  rw.MatchFlags.PosixSyntax,
				LongestMatch: rw.MatchFlags.LongestMatch,
				Literal:      rw.MatchFlags.Literal,
				NeverCapture: rw.MatchFlags.NeverCapture,
				PerlClasses:  rw.MatchFlags.PerlClasses,
				WordBoundary: rw.MatchFlags.WordBoundary,
			}
			if rw.MatchFlags.MaxMem != nil &&
				*rw.MatchFlags.MaxMem != 0 {

				vclRw.MatchFlags.MaxMem = *rw.MatchFlags.MaxMem
			}
			if rw.MatchFlags.CaseSensitive == nil {
				vclRw.MatchFlags.CaseSensitive = true
			} else {
				vclRw.MatchFlags.CaseSensitive =
					*rw.MatchFlags.CaseSensitive
			}
			switch rw.MatchFlags.Anchor {
			case vcr_v1alpha1.None:
				vclRw.MatchFlags.Anchor = vcl.None
			case vcr_v1alpha1.Start:
				vclRw.MatchFlags.Anchor = vcl.Start
			case vcr_v1alpha1.Both:
				vclRw.MatchFlags.Anchor = vcl.Both
			default:
				vclRw.MatchFlags.Anchor = vcl.None
			}
		}
		spec.Rewrites[i] = vclRw
	}
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

	var vcfg *vcr_v1alpha1.VarnishConfig
	worker.log.Debugf("Listing VarnishConfigs in namespace %s",
		worker.namespace)
	vcfgs, err := worker.vcfg.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, v := range vcfgs {
		worker.log.Debugf("VarnishConfig: %s/%s: %+v", v.Namespace,
			v.Name, v)
		for _, svcName := range v.Spec.Services {
			if svcName == svc.Name {
				vcfg = v
				break
			}
		}
	}
	if vcfg != nil {
		worker.log.Infof("Found VarnishConfig %s/%s for Varnish "+
			"Service %s/%s", vcfg.Namespace, vcfg.Name,
			svc.Namespace, svc.Name)
		if err = worker.configSharding(&vclSpec, vcfg, svc); err != nil {
			return err
		}
		if err = worker.configAuth(&vclSpec, vcfg); err != nil {
			return err
		}
		if err = worker.configACL(&vclSpec, vcfg); err != nil {
			return err
		}
		if err = worker.configRewrites(&vclSpec, vcfg); err != nil {
			return err
		}
		vclSpec.VCL = vcfg.Spec.VCL
	} else {
		worker.log.Infof("Found no VarnishConfigs for Varnish Service "+
			"%s/%s", svc.Namespace, svc.Name)
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
	}
	worker.log.Debugf("Updated Ingress key=%s uuid=%s svc=%s: %+v", ingKey,
		string(ing.ObjectMeta.UID), svcKey, vclSpec)
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
		syncCounters.WithLabelValues(worker.namespace, "Ingress",
			"Ignore").Inc()
		return nil
	}
	return worker.addOrUpdateIng(ing)
}

func (worker *NamespaceWorker) addIng(key string) error {
	return worker.syncIng(key)
}

func (worker *NamespaceWorker) updateIng(key string) error {
	return worker.syncIng(key)
}

func (worker *NamespaceWorker) deleteIng(obj interface{}) error {
	ing, ok := obj.(*extensions.Ingress)
	if !ok || ing == nil {
		// XXX should clean up Varnish config nevertheless
		worker.log.Warnf("Delete Ingress: not found: %v", obj)
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
