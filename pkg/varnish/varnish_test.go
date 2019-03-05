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

package varnish

import (
	"fmt"
	"strings"
	"testing"

	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"
)

func TestVarnishAdmError(t *testing.T) {
	vadmErr := VarnishAdmError{
		addr: "123.45.67.89:4711",
		err:  fmt.Errorf("Error message"),
	}
	err := vadmErr.Error()
	want := "123.45.67.89:4711: Error message"
	if err != want {
		t.Errorf("VarnishAdmError.Error() want=%s got=%s", want, err)
	}

	vadmErrs := VarnishAdmErrors{
		vadmErr,
		VarnishAdmError{
			addr: "98.76.54.321:815",
			err:  fmt.Errorf("Error 2"),
		},
		VarnishAdmError{
			addr: "192.0.2.255:80",
			err:  fmt.Errorf("Error 3"),
		},
	}
	err = vadmErrs.Error()
	want = "[{123.45.67.89:4711: Error message}{98.76.54.321:815: Error 2}" +
		"{192.0.2.255:80: Error 3}]"
	if err != want {
		t.Errorf("VarnishAdmErrors.Error() want=%s got=%s", want, err)
	}
}

// Test data for HasConfig()

var teaSvc = vcl.Service{
	Name: "tea-svc",
	Addresses: []vcl.Address{
		{
			IP:   "192.0.2.1",
			Port: 80,
		},
		{
			IP:   "192.0.2.2",
			Port: 80,
		},
		{
			IP:   "192.0.2.3",
			Port: 80,
		},
	},
}

var coffeeSvc = vcl.Service{
	Name: "coffee-svc",
	Addresses: []vcl.Address{
		{
			IP:   "192.0.2.4",
			Port: 80,
		},
		{
			IP:   "192.0.2.5",
			Port: 80,
		},
	},
}

var cafeSpec = vcl.Spec{
	DefaultService: vcl.Service{},
	Rules: []vcl.Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]vcl.Service{
			"/tea":    teaSvc,
			"/coffee": coffeeSvc,
		},
	}},
	AllServices: map[string]vcl.Service{
		"tea-svc":    teaSvc,
		"coffee-svc": coffeeSvc,
	},
}

var teaSvcShuf = vcl.Service{
	Name: "tea-svc",
	Addresses: []vcl.Address{
		{
			IP:   "192.0.2.3",
			Port: 80,
		},
		{
			IP:   "192.0.2.1",
			Port: 80,
		},
		{
			IP:   "192.0.2.2",
			Port: 80,
		},
	},
}

var coffeeSvcShuf = vcl.Service{
	Name: "coffee-svc",
	Addresses: []vcl.Address{
		{
			IP:   "192.0.2.5",
			Port: 80,
		},
		{
			IP:   "192.0.2.4",
			Port: 80,
		},
	},
}

var cafeSpecShuf = vcl.Spec{
	DefaultService: vcl.Service{},
	Rules: []vcl.Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]vcl.Service{
			"/coffee": coffeeSvcShuf,
			"/tea":    teaSvcShuf,
		},
	}},
	AllServices: map[string]vcl.Service{
		"coffee-svc": coffeeSvcShuf,
		"tea-svc":    teaSvcShuf,
	},
}

var ingsMeta = map[string]Meta{
	"default/cafe": Meta{
		Key: "default/cafe",
		UID: "123e4567-e89b-12d3-a456-426655440000",
		Ver: "123456",
	},
	"ns/name": Meta{
		Key: "ns/name",
		UID: "00112233-4455-6677-8899-aabbccddeeff",
		Ver: "654321",
	},
	"kube-system/ingress": Meta{
		Key: "kube-system/ingress",
		UID: "6ba7b812-9dad-11d1-80b4-00c04fd430c8",
		Ver: "987654",
	},
}

var bcfgsMeta = map[string]Meta{
	"coffee-svc": Meta{
		Key: "default/coffee-svc-cfg",
		UID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Ver: "010101",
	},
	"tea-svc": Meta{
		Key: "ns/tea-svc-cfg",
		UID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		Ver: "909090",
	},
}

var vcfgMeta = Meta{
	Key: "default/varnish-cfg",
	UID: "6ba7b814-9dad-11d1-80b4-00c04fd430c8",
	Ver: "37337",
}

func TestHasConfig(t *testing.T) {
	spec := vclSpec{
		spec: cafeSpec,
		ings: ingsMeta,
		vcfg: vcfgMeta,
		bcfg: bcfgsMeta,
	}
	vSvc := varnishSvc{
		spec:      &spec,
		cfgLoaded: true,
	}
	vc := VarnishController{
		svcs: map[string]*varnishSvc{"default/cafe-ingress": &vSvc},
	}
	svcKey := "default/cafe-ingress"

	if !vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, vcfgMeta, bcfgsMeta) {

		t.Errorf("HasConfig() got:false want:true")
	}

	if vc.HasConfig("ns/name", cafeSpecShuf, ingsMeta, vcfgMeta,
		bcfgsMeta) {

		t.Errorf("HasConfig(unknown Service) got:true want:false")
	}

	vSvc.cfgLoaded = false
	if vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, vcfgMeta, bcfgsMeta) {

		t.Errorf("HasConfig(cfgLoaded=false) got:true want:false")
	}
	vSvc.cfgLoaded = true

	otherVcfg := vcfgMeta
	otherVcfg.Ver = "37338"
	if vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, otherVcfg, bcfgsMeta) {

		t.Errorf("HasConfig(changed VarnishConfig) got:true want:false")
	}

	otherIngs := make(map[string]Meta)
	for k, v := range ingsMeta {
		otherIngs[k] = v
	}
	otherIngs["key"] = Meta{}
	if vc.HasConfig(svcKey, cafeSpecShuf, otherIngs, vcfgMeta, bcfgsMeta) {

		t.Errorf("HasConfig(more Ingresses) got:true want:false")
	}
	delete(otherIngs, "key")

	otherIngs["default/cafe"] = Meta{
		Key: "default/cafe",
		UID: "123e4567-e89b-12d3-a456-426655440000",
		Ver: "123457",
	}
	if vc.HasConfig(svcKey, cafeSpecShuf, otherIngs, vcfgMeta, bcfgsMeta) {

		t.Errorf("HasConfig(changed Ingresses) got:true want:false")
	}

	delete(otherIngs, "default/cafe")
	if vc.HasConfig(svcKey, cafeSpecShuf, otherIngs, vcfgMeta, bcfgsMeta) {

		t.Errorf("HasConfig(fewer Ingresses) got:true want:false")
	}

	otherBcfgs := make(map[string]Meta)
	for k, v := range bcfgsMeta {
		otherBcfgs[k] = v
	}
	otherBcfgs["key"] = Meta{}
	if vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, vcfgMeta, otherBcfgs) {

		t.Errorf("HasConfig(more BackendConfigs) got:true want:false")
	}
	delete(otherBcfgs, "key")

	otherBcfgs["coffee-svc"] = Meta{
		Key: "default/coffee-svc-cfg",
		UID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Ver: "010102",
	}
	if vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, vcfgMeta, otherBcfgs) {

		t.Errorf("HasConfig(changed BackendConfigs) got:true want:false")
	}

	delete(otherBcfgs, "coffee-svc")
	if vc.HasConfig(svcKey, cafeSpecShuf, ingsMeta, vcfgMeta, otherBcfgs) {

		t.Errorf("HasConfig(fewer BackendConfigs) got:true want:false")
	}
}

func TestConfigName(t *testing.T) {
	spec := vclSpec{spec: cafeSpec}
	name1 := spec.configName()
	if !strings.HasPrefix(name1, ingressPrefix) {
		t.Errorf("configName(): name %s does not have prefix %s",
			name1, ingressPrefix)
	}

	spec = vclSpec{spec: cafeSpecShuf}
	name2 := spec.configName()
	if !strings.HasPrefix(name2, ingressPrefix) {
		t.Errorf("configName(): name %s does not have prefix %s",
			name2, ingressPrefix)
	}

	if name1 != name2 {
		t.Errorf("configName(): equivalent specs have different names:"+
			"'%s' '%s'", name1, name2)
	}
}
