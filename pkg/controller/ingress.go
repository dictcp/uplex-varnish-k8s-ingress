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
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"

	"k8s.io/client-go/tools/cache"

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

	svcs, err := worker.listers.svc.List(varnishIngressSelector)
	if err != nil {
		return nil, err
	}
	if varnishSvc, exists := ing.Annotations[varnishSvcKey]; exists {
		worker.log.Tracef("Ingress %s/%s has annotation %s:%s",
			ing.Namespace, ing.Name, varnishSvcKey, varnishSvc)
		targetNs, targetSvc, err :=
			cache.SplitMetaNamespaceKey(varnishSvc)
		if err != nil {
			return nil, err
		}
		if targetNs == "" {
			targetNs = worker.namespace
		}
		for _, svc := range svcs {
			if svc.Namespace == targetNs && svc.Name == targetSvc {
				return svc, nil
			}
		}
		worker.log.Tracef("Ingress %s/%s: Varnish Service %s not found",
			ing.Namespace, ing.Name, varnishSvc)
		return nil, nil
	}
	worker.log.Tracef("Ingress %s/%s does not have annotation %s",
		ing.Namespace, ing.Name, varnishSvcKey)
	if len(svcs) == 1 {
		worker.log.Tracef("Exactly one Varnish Ingress Service "+
			"cluster-wide: %s", svcs[0])
		return svcs[0], nil
	}
	svcs, err = worker.svc.List(varnishIngressSelector)
	if err != nil {
		return nil, err
	}
	if len(svcs) == 1 {
		worker.log.Tracef("Exactly one Varnish Ingress Service "+
			"in namespace %s: %s", worker.namespace, svcs[0])
		return svcs[0], nil
	}
	return nil, nil
}

func (worker *NamespaceWorker) getIngsForVarnishSvc(
	svc *api_v1.Service) ([]*extensions.Ingress, error) {

	ings, err := worker.listers.ing.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	allVarnishSvcs, err := worker.listers.svc.List(varnishIngressSelector)
	if err != nil {
		return nil, err
	}
	nsVarnishSvcs, err := worker.svc.List(varnishIngressSelector)
	if err != nil {
		return nil, err
	}
	ings4Svc := make([]*extensions.Ingress, 0)
	for _, ing := range ings {
		if !worker.isVarnishIngress(ing) {
			continue
		}
		namespace := ing.Namespace
		if namespace == "" {
			namespace = "default"
		}
		if ingSvc, exists := ing.Annotations[varnishSvcKey]; exists {
			targetNs, targetSvc, err :=
				cache.SplitMetaNamespaceKey(ingSvc)
			if err != nil {
				return nil, err
			}
			if targetNs == "" {
				targetNs = namespace
			}
			if targetNs == svc.Namespace && targetSvc == svc.Name {
				ings4Svc = append(ings4Svc, ing)
			}
		} else if len(allVarnishSvcs) == 1 {
			ings4Svc = append(ings4Svc, ing)
		} else if ing.Namespace == svc.Namespace &&
			len(nsVarnishSvcs) == 1 {

			ings4Svc = append(ings4Svc, ing)
		}
	}
	return ings4Svc, nil
}

func ingMergeError(ings []*extensions.Ingress) error {
	host2ing := make(map[string]*extensions.Ingress)
	var ingWdefBackend *extensions.Ingress
	for _, ing := range ings {
		if ing.Spec.Backend != nil {
			if ingWdefBackend != nil {
				return fmt.Errorf("Default backend configured "+
					"in more than one Ingress: %s/%s and "+
					"%s/%s", ing.Namespace, ing.Name,
					ingWdefBackend.Namespace,
					ingWdefBackend.Name)
			}
			ingWdefBackend = ing
		}
		for _, rule := range ing.Spec.Rules {
			if otherIng, exists := host2ing[rule.Host]; exists {
				return fmt.Errorf("Host '%s' named in rules "+
					"for more than one Ingress: %s/%s and "+
					"%s/%s", rule.Host, otherIng.Namespace,
					otherIng.Name, ing.Namespace, ing.Name)
			}
			host2ing[rule.Host] = ing
		}
	}
	return nil
}

func (worker *NamespaceWorker) ingBackend2Addrs(namespace string,
	backend extensions.IngressBackend) (addrs []vcl.Address, err error) {

	nsLister := worker.listers.svc.Services(namespace)
	svc, err := nsLister.Get(backend.ServiceName)
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

func (worker *NamespaceWorker) getVCLSvc(svcNamespace string, svcName string,
	addrs []vcl.Address) (vcl.Service, *vcr_v1alpha1.BackendConfig, error) {

	if svcNamespace == "" {
		svcNamespace = "default"
	}
	vclSvc := vcl.Service{
		Name:      svcNamespace + "/" + svcName,
		Addresses: addrs,
	}
	nsLister := worker.listers.bcfg.BackendConfigs(svcNamespace)
	bcfgs, err := nsLister.List(labels.Everything())
	if err != nil {
		if errors.IsNotFound(err) {
			return vclSvc, nil, nil
		}
		return vclSvc, nil, err
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
		return vclSvc, nil, nil
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
	return vclSvc, bcfg, nil
}

func (worker *NamespaceWorker) ings2VCLSpec(
	ings []*extensions.Ingress) (vcl.Spec,
	map[string]*vcr_v1alpha1.BackendConfig, error) {
	vclSpec := vcl.Spec{}
	vclSpec.AllServices = make(map[string]vcl.Service)
	bcfgs := make(map[string]*vcr_v1alpha1.BackendConfig)
	for _, ing := range ings {
		namespace := ing.Namespace
		if namespace == "" {
			namespace = "default"
		}
		if ing.Spec.TLS != nil && len(ing.Spec.TLS) > 0 {
			worker.log.Warnf("TLS config currently ignored in "+
				"Ingress %s/%s", namespace, ing.Name)
		}
		if ing.Spec.Backend != nil {
			if vclSpec.DefaultService.Name != "" {
				panic("More than one Ingress default backend")
			}
			backend := ing.Spec.Backend
			addrs, err := worker.ingBackend2Addrs(namespace,
				*backend)
			if err != nil {
				return vclSpec, bcfgs, err
			}
			vclSvc, bcfg, err := worker.getVCLSvc(namespace,
				backend.ServiceName, addrs)
			if err != nil {
				return vclSpec, bcfgs, err
			}
			vclSpec.DefaultService = vclSvc
			vclSpec.AllServices[namespace+"/"+backend.ServiceName] = vclSvc
			if bcfg != nil {
				bcfgs[vclSvc.Name] = bcfg
			}
		}
		for _, rule := range ing.Spec.Rules {
			// XXX this should not be an error
			if rule.Host == "" {
				return vclSpec, bcfgs,
					fmt.Errorf("Ingress rule contains empty Host")
			}
			vclRule := vcl.Rule{Host: rule.Host}
			vclRule.PathMap = make(map[string]vcl.Service)
			if rule.IngressRuleValue.HTTP == nil {
				vclSpec.Rules = append(vclSpec.Rules, vclRule)
				continue
			}
			for _, path := range rule.IngressRuleValue.HTTP.Paths {
				addrs, err := worker.ingBackend2Addrs(
					namespace, path.Backend)
				if err != nil {
					return vclSpec, bcfgs, err
				}
				vclSvc, bcfg, err := worker.getVCLSvc(namespace,
					path.Backend.ServiceName, addrs)
				if err != nil {
					return vclSpec, bcfgs, err
				}
				vclRule.PathMap[path.Path] = vclSvc
				vclSpec.AllServices[namespace+"/"+
					path.Backend.ServiceName] = vclSvc
				if bcfg != nil {
					bcfgs[vclSvc.Name] = bcfg
				}
			}
			vclSpec.Rules = append(vclSpec.Rules, vclRule)
		}
	}
	return vclSpec, bcfgs, nil
}

func (worker *NamespaceWorker) configSharding(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig, svc *api_v1.Service) error {

	if vcfg.Spec.SelfSharding == nil {
		worker.log.Tracef("No cluster shard configuration for Service "+
			"%s/%s", svc.Namespace, svc.Name)
		return nil
	}

	worker.log.Tracef("Set cluster shard configuration for Service %s/%s",
		svc.Namespace, svc.Name)

	endps, err := worker.getServiceEndpoints(svc)
	if err != nil {
		return fmt.Errorf("Error getting endpoints for service %s/%s: "+
			"%v", svc.Namespace, svc.Name, err)
	}
	worker.log.Tracef("Endpoints for shard configuration: %+v", endps)

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
	worker.log.Tracef("Pods for shard configuration: %+v", pods.Items)

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
	worker.log.Tracef("Node configuration for self-sharding in Service "+
		"%s/%s: %+v", svc.Namespace, svc.Name, spec.ShardCluster.Nodes)

	cfgSpec := vcfg.Spec.SelfSharding
	probe := getVCLProbe(&cfgSpec.Probe)
	spec.ShardCluster.Probe = *probe
	if cfgSpec.Max2ndTTL != "" {
		spec.ShardCluster.MaxSecondaryTTL = cfgSpec.Max2ndTTL
	}
	worker.log.Tracef("Spec configuration for self-sharding in Service "+
		"%s/%s: %+v", svc.Namespace, svc.Name, spec.ShardCluster)
	return nil
}

func configComparison(cmp vcr_v1alpha1.CompareType) (vcl.CompareType, bool) {
	switch cmp {
	case vcr_v1alpha1.Equal:
		return vcl.Equal, false
	case vcr_v1alpha1.NotEqual:
		return vcl.Equal, true
	case vcr_v1alpha1.Match:
		return vcl.Match, false
	case vcr_v1alpha1.NotMatch:
		return vcl.Match, true
	case vcr_v1alpha1.Prefix:
		return vcl.Prefix, false
	case vcr_v1alpha1.NotPrefix:
		return vcl.Prefix, true
	case vcr_v1alpha1.Exists:
		return vcl.Exists, false
	case vcr_v1alpha1.NotExists:
		return vcl.Exists, true
	case vcr_v1alpha1.Greater:
		return vcl.Greater, false
	case vcr_v1alpha1.GreaterEqual:
		return vcl.GreaterEqual, false
	case vcr_v1alpha1.Less:
		return vcl.Less, false
	case vcr_v1alpha1.LessEqual:
		return vcl.LessEqual, false
	default:
		return vcl.Equal, false
	}
}

func configConditions(vclConds []vcl.MatchTerm,
	vcfgConds []vcr_v1alpha1.Condition) {

	if len(vclConds) != len(vcfgConds) {
		panic("configConditions: unequal slice lengths")
	}
	for i, cond := range vcfgConds {
		vclMatch := vcl.MatchTerm{
			Comparand: cond.Comparand,
			Value:     cond.Value,
		}
		vclMatch.Compare, vclMatch.Negate =
			configComparison(cond.Compare)
		vclConds[i] = vclMatch
	}
}

func (worker *NamespaceWorker) configAuth(spec *vcl.Spec,
	vcfg *vcr_v1alpha1.VarnishConfig) error {

	if len(vcfg.Spec.Auth) == 0 {
		worker.log.Infof("No Auth spec found for VarnishConfig %s/%s",
			vcfg.Namespace, vcfg.Name)
		return nil
	}
	worker.log.Tracef("VarnishConfig %s/%s: configure %d VCL auths",
		vcfg.Namespace, vcfg.Name, len(vcfg.Spec.Auth))
	spec.Auths = make([]vcl.Auth, 0, len(vcfg.Spec.Auth))
	for _, auth := range vcfg.Spec.Auth {
		worker.log.Tracef("VarnishConfig %s/%s configuring VCL auth "+
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
		worker.log.Tracef("VarnishConfig %s/%s configure %d "+
			"credentials for realm %s", vcfg.Namespace, vcfg.Name,
			len(secret.Data), auth.Realm)
		vclAuth := vcl.Auth{
			Realm:       auth.Realm,
			Credentials: make([]string, 0, len(secret.Data)),
			Conditions:  make([]vcl.MatchTerm, len(auth.Conditions)),
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
			worker.log.Tracef("VarnishConfig %s/%s: add cred %s "+
				"for realm %s to VCL config", vcfg.Namespace,
				vcfg.Name, cred, vclAuth.Realm)
			vclAuth.Credentials = append(vclAuth.Credentials, cred)
		}
		configConditions(vclAuth.Conditions, auth.Conditions)
		worker.log.Tracef("VarnishConfig %s/%s add VCL auth config: "+
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
		worker.log.Tracef("ACL %s: %+v", acl.Name, acl)
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
		configConditions(vclACL.Conditions, acl.Conditions)
		if acl.ResultHdr != nil {
			worker.log.Tracef("ACL %s: ResultHdr=%+v", acl.Name,
				*acl.ResultHdr)
			vclACL.ResultHdr.Header = acl.ResultHdr.Header
			vclACL.ResultHdr.Success = acl.ResultHdr.Success
			vclACL.ResultHdr.Failure = acl.ResultHdr.Failure
		}
		worker.log.Tracef("VarnishConfig %s/%s add VCL ACL config: "+
			"%+v", vcfg.Namespace, vcfg.Name, vclACL)
		spec.ACLs[i] = vclACL
	}
	return nil
}

func configMatchFlags(flags vcr_v1alpha1.MatchFlagsType) vcl.MatchFlagsType {
	vclFlags := vcl.MatchFlagsType{
		UTF8:         flags.UTF8,
		PosixSyntax:  flags.PosixSyntax,
		LongestMatch: flags.LongestMatch,
		Literal:      flags.Literal,
		NeverCapture: flags.NeverCapture,
		PerlClasses:  flags.PerlClasses,
		WordBoundary: flags.WordBoundary,
	}
	if flags.MaxMem != nil && *flags.MaxMem != 0 {
		vclFlags.MaxMem = *flags.MaxMem
	}
	if flags.CaseSensitive == nil {
		vclFlags.CaseSensitive = true
	} else {
		vclFlags.CaseSensitive = *flags.CaseSensitive
	}
	switch flags.Anchor {
	case vcr_v1alpha1.None:
		vclFlags.Anchor = vcl.None
	case vcr_v1alpha1.Start:
		vclFlags.Anchor = vcl.Start
	case vcr_v1alpha1.Both:
		vclFlags.Anchor = vcl.Both
	default:
		vclFlags.Anchor = vcl.None
	}
	return vclFlags
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
		worker.log.Tracef("Rewrite: %v", rw)
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

		if rw.Compare == "" {
			rw.Compare = vcr_v1alpha1.Match
		}
		vclRw.Compare, vclRw.Negate = configComparison(rw.Compare)

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

		switch rw.Select {
		case vcr_v1alpha1.Unique:
			vclRw.Select = vcl.Unique
		case vcr_v1alpha1.First:
			vclRw.Select = vcl.First
		case vcr_v1alpha1.Last:
			vclRw.Select = vcl.Last
		case vcr_v1alpha1.Exact:
			vclRw.Select = vcl.Exact
		case vcr_v1alpha1.Longest:
			vclRw.Select = vcl.Longest
		case vcr_v1alpha1.Shortest:
			vclRw.Select = vcl.Shortest
		default:
			vclRw.Select = vcl.Unique
		}

		if rw.MatchFlags != nil {
			vclRw.MatchFlags = configMatchFlags(*rw.MatchFlags)
		} else {
			vclRw.MatchFlags.CaseSensitive = true
		}
		spec.Rewrites[i] = vclRw
	}
	return nil
}

func (worker *NamespaceWorker) configReqDisps(spec *vcl.Spec,
	reqDisps []vcr_v1alpha1.RequestDispSpec, kind, namespace, name string) {

	if len(reqDisps) == 0 {
		worker.log.Infof("No request disposition specs found for %s "+
			"%s/%s", kind, namespace, name)
		return
	}
	worker.log.Infof("Configuring request dispositions for %s %s/%s",
		kind, namespace, name)
	spec.Dispositions = make([]vcl.DispositionSpec, len(reqDisps))
	for i, disp := range reqDisps {
		worker.log.Tracef("ReqDisposition: %+v", disp)
		vclDisp := vcl.DispositionSpec{
			Conditions: make([]vcl.Condition, len(disp.Conditions)),
		}
		for j, cond := range disp.Conditions {
			vclCond := vcl.Condition{
				Comparand: cond.Comparand,
			}
			if len(cond.Values) > 0 {
				vclCond.Values = make([]string, len(cond.Values))
				copy(vclCond.Values, cond.Values)
			}
			if cond.Count != nil {
				count := uint(*cond.Count)
				vclCond.Count = &count
			}
			vclCond.Compare, vclCond.Negate =
				configComparison(cond.Compare)
			if cond.MatchFlags != nil {
				vclCond.MatchFlags = configMatchFlags(
					*cond.MatchFlags)
			} else {
				vclCond.MatchFlags.CaseSensitive = true
			}
			vclDisp.Conditions[j] = vclCond
		}
		vclDisp.Disposition.Action = vcl.RecvReturn(
			disp.Disposition.Action)
		if disp.Disposition.Action == vcr_v1alpha1.RecvSynth {
			vclDisp.Disposition.Status = uint16(
				*disp.Disposition.Status)
		}
		spec.Dispositions[i] = vclDisp
	}
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
	worker.log.Infof("Ingress %s configured for Varnish Service %s", ingKey,
		svcKey)

	ings, err := worker.getIngsForVarnishSvc(svc)
	if err != nil {
		return nil
	}
	if len(ings) == 0 {
		worker.log.Infof("No Ingresses to be implemented by Varnish "+
			"Service %s, setting to not ready", svcKey)
		return worker.vController.SetNotReady(svcKey)
	}

	ingNames := make([]string, len(ings))
	for i, ingress := range ings {
		ingNames[i] = ingress.Namespace + "/" + ingress.Name
	}
	worker.log.Infof("Ingresses implemented by Varnish Service %s: %v",
		svcKey, ingNames)
	vclSpec, bcfgs, err := worker.ings2VCLSpec(ings)
	if err != nil {
		return err
	}
	worker.log.Tracef("VCL spec generated from the Ingresses: %v", vclSpec)

	var vcfg *vcr_v1alpha1.VarnishConfig
	worker.log.Tracef("Listing VarnishConfigs in namespace %s",
		worker.namespace)
	vcfgs, err := worker.vcfg.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, v := range vcfgs {
		worker.log.Tracef("VarnishConfig: %s/%s: %+v", v.Namespace,
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
		worker.configReqDisps(&vclSpec, vcfg.Spec.ReqDispositions,
			vcfg.Kind, vcfg.Namespace, vcfg.Name)
		vclSpec.VCL = vcfg.Spec.VCL
	} else {
		worker.log.Infof("Found no VarnishConfigs for Varnish Service "+
			"%s/%s", svc.Namespace, svc.Name)
	}

	ingsMeta := make(map[string]varnish.Meta)
	for _, ing := range ings {
		metaDatum := varnish.Meta{
			Key: ing.Namespace + "/" + ing.Name,
			UID: string(ing.UID),
			Ver: ing.ResourceVersion,
		}
		ingsMeta[metaDatum.Key] = metaDatum
	}
	var vcfgMeta varnish.Meta
	if vcfg != nil {
		vcfgMeta = varnish.Meta{
			Key: vcfg.Namespace + "/" + vcfg.Name,
			UID: string(vcfg.UID),
			Ver: vcfg.ResourceVersion,
		}
	}
	bcfgMeta := make(map[string]varnish.Meta)
	for name, bcfg := range bcfgs {
		bcfgMeta[name] = varnish.Meta{
			Key: bcfg.Namespace + "/" + bcfg.Name,
			UID: string(bcfg.UID),
			Ver: bcfg.ResourceVersion,
		}
	}
	worker.log.Tracef("Check if config is loaded: hash=%s "+
		"ingressMetaData=%+v vcfgMetaData=%+v bcfgMetaData=%+v",
		vclSpec.Canonical().DeepHash(), ingsMeta, vcfgMeta, bcfgMeta)
	if worker.vController.HasConfig(svcKey, vclSpec, ingsMeta, vcfgMeta,
		bcfgMeta) {
		worker.log.Infof("Varnish Service %s: config already "+
			"loaded: hash=%s", svcKey,
			vclSpec.Canonical().DeepHash())
		return nil
	}
	worker.log.Tracef("Update config svc=%s ingressMetaData=%+v "+
		"vcfgMetaData=%+v bcfgMetaData=%+v: %+v", svcKey, ingsMeta,
		vcfgMeta, bcfgMeta, vclSpec)
	err = worker.vController.Update(svcKey, vclSpec, ingsMeta, vcfgMeta,
		bcfgMeta)
	if err != nil {
		return err
	}
	worker.log.Tracef("Updated config svc=%s ingressMetaData=%+v "+
		"vcfgMetaData=%+v bcfgMetaData=%+v: %+v", svcKey, ingsMeta,
		vcfgMeta, bcfgMeta, vclSpec)
	return nil
}

// We only handle Ingresses with the class annotation with the value
// given as the "class" flag (default "varnish").
func (worker *NamespaceWorker) isVarnishIngress(ing *extensions.Ingress) bool {
	class, exists := ing.Annotations[ingressClassKey]
	return exists && class == worker.ingClass
}

func (worker *NamespaceWorker) syncIng(key string) error {
	nsKey := worker.namespace + "/" + key
	worker.log.Info("Syncing Ingress:", nsKey)
	ing, err := worker.ing.Get(key)
	if err != nil {
		return err
	}

	if !worker.isVarnishIngress(ing) {
		worker.log.Infof("Ignoring Ingress %s/%s, Annotation '%v' "+
			"absent or is not '%s'", ing.Namespace, ing.Name,
			ingressClassKey, worker.ingClass)
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
		worker.log.Warnf("Delete Ingress: not found: %v", obj)
		return nil
	}
	return worker.addOrUpdateIng(ing)
}
