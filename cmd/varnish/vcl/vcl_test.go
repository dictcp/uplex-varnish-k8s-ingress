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

package vcl

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
)

var teaSvc = Service{
	Name: "tea-svc",
	Addresses: []Address{
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

var coffeeSvc = Service{
	Name: "coffee-svc",
	Addresses: []Address{
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

var cafeSpec = Spec{
	DefaultService: Service{},
	Rules: []Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]Service{
			"/tea":    teaSvc,
			"/coffee": coffeeSvc,
		},
	}},
	AllServices: map[string]Service{
		"tea-svc":    teaSvc,
		"coffee-svc": coffeeSvc,
	},
}

func TestTemplate(t *testing.T) {
	var buf bytes.Buffer
	if err := Tmpl.Execute(&buf, cafeSpec); err != nil {
		t.Error("Execute():", err)
	}

	goldpath := filepath.Join("testdata", "ingressrule.golden")
	gold, err := ioutil.ReadFile(goldpath)
	if err != nil {
		t.Fatalf("Error reading %s: %v", goldpath, err)
	}
	if !bytes.Equal(buf.Bytes(), gold) {
		t.Errorf("Generated VCL for IngressSpec does not match gold "+
			"file: %s", goldpath)
		if testing.Verbose() {
			t.Log("Generated VCL:", string(buf.Bytes()))
			t.Log(goldpath, ":", string(gold))
		}
	}
}

var coffeeSvc3 = Service{
	Name: "coffee-svc",
	Addresses: []Address{
		{
			IP:   "192.0.2.4",
			Port: 80,
		},
		{
			IP:   "192.0.2.5",
			Port: 80,
		},
		{
			IP:   "192.0.2.6",
			Port: 80,
		},
	},
}

var cafeSpec2 = Spec{
	DefaultService: Service{},
	Rules: []Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]Service{
			"/tea":    teaSvc,
			"/coffee": coffeeSvc3,
		},
	}},
	AllServices: map[string]Service{
		"tea-svc":    teaSvc,
		"coffee-svc": coffeeSvc3,
	},
}

func TestDeepHash(t *testing.T) {
	if cafeSpec.DeepHash() == cafeSpec2.DeepHash() {
		t.Errorf("DeepHash(): Distinct specs have equal hashes")
		if testing.Verbose() {
			t.Logf("spec1: %+v", cafeSpec)
			t.Logf("spec2: %+v", cafeSpec2)
			t.Logf("hash: %0x", cafeSpec.DeepHash())
		}
	}
}

var teaSvcShuf = Service{
	Name: "tea-svc",
	Addresses: []Address{
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

var coffeeSvcShuf = Service{
	Name: "coffee-svc",
	Addresses: []Address{
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

var cafeSpecShuf = Spec{
	DefaultService: Service{},
	Rules: []Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]Service{
			"/coffee": coffeeSvcShuf,
			"/tea":    teaSvcShuf,
		},
	}},
	AllServices: map[string]Service{
		"coffee-svc": coffeeSvcShuf,
		"tea-svc":    teaSvcShuf,
	},
}

func TestCanoncial(t *testing.T) {
	canonCafe := cafeSpec.Canonical()
	canonShuf := cafeSpecShuf.Canonical()
	if !reflect.DeepEqual(canonCafe, canonShuf) {
		t.Error("Canonical(): Equivalent VCL specs not deeply equal")
		if testing.Verbose() {
			t.Log("Canonical cafe:", canonCafe)
			t.Log("Canonical shuffled cafe:", canonShuf)
		}
	}
	if canonCafe.DeepHash() != canonShuf.DeepHash() {
		t.Error("Canonical(): Unequal hashes for equivalent specs")
		if testing.Verbose() {
			t.Logf("spec1 canonical: %+v", canonCafe)
			t.Logf("spec1 hash: %0x", canonCafe.DeepHash())
			t.Logf("spec2 canonical: %+v", canonShuf)
			t.Logf("spec2 hash: %0x", canonShuf.DeepHash())
		}
	}
}
