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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"text/template"
)

func cmpGold(got []byte, goldfile string) (bool, error) {
	goldpath := filepath.Join("testdata", goldfile)
	gold, err := ioutil.ReadFile(goldpath)
	if err != nil {
		return false, err
	}
	return bytes.Equal(got, gold), nil
}

func TestMain(m *testing.M) {
	tmplDir := ""
	tmplEnv, exists := os.LookupEnv("TEMPLATE_DIR")
	if !exists {
		tmplDir = tmplEnv
	}
	if err := InitTemplates(tmplDir); err != nil {
		fmt.Printf("Cannot parse templates: %v\n", err)
		os.Exit(-1)
	}
	code := m.Run()
	os.Exit(code)
}

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

func TestIngressTemplate(t *testing.T) {
	var buf bytes.Buffer
	gold := "ingressrule.golden"
	if err := ingressTmpl.Execute(&buf, cafeSpec); err != nil {
		t.Fatal("Execute():", err)
	}
	ok, err := cmpGold(buf.Bytes(), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL for IngressSpec does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", buf.String())
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

var varnishCluster = ShardCluster{
	Nodes: []Service{
		Service{
			Name:      "varnish-8445d4f7f-z2b9p",
			Addresses: []Address{{"172.17.0.12", 80}},
		},
		Service{
			Name:      "varnish-8445d4f7f-k22dn",
			Addresses: []Address{{"172.17.0.13", 80}},
		},
		Service{
			Name:      "varnish-8445d4f7f-ldljf",
			Addresses: []Address{{"172.17.0.14", 80}},
		},
	},
	Probe: Probe{
		Timeout:   "2s",
		Interval:  "5s",
		Initial:   "2",
		Window:    "8",
		Threshold: "3",
	},
	MaxSecondaryTTL: "5m",
}

func TestShardTemplate(t *testing.T) {
	var buf bytes.Buffer
	gold := "shard.golden"
	tmplName := "self-shard.tmpl"

	tmpl, err := template.New(tmplName).Funcs(fMap).ParseFiles(tmplName)
	if err != nil {
		t.Error("Cannot parse shard template:", err)
		return
	}
	if err := tmpl.Execute(&buf, varnishCluster); err != nil {
		t.Error("cluster template Execute():", err)
		return
	}
	ok, err := cmpGold(buf.Bytes(), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL for self-sharding does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", buf.String())
		}
	}
}

func TestGetSrc(t *testing.T) {
	gold := "ingress_shard.golden"
	cafeSpec.ShardCluster = varnishCluster
	src, err := cafeSpec.GetSrc()
	if err != nil {
		t.Error("Spec.GetSrc():", err)
		return
	}
	ok, err := cmpGold([]byte(src), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL from GetSrc() does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", src)
		}
	}
}

var auths = Spec{
	Auths: []Auth{
		{
			Realm:  "foo",
			Status: Basic,
			Credentials: []string{
				"QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
				"QWxhZGRpbjpPcGVuU2VzYW1l",
			},
		},
		{
			Realm:  "bar",
			Status: Proxy,
			Credentials: []string{
				"Zm9vOmJhcg==",
				"YmF6OnF1dXg=",
			},
			UTF8: false,
		},
		{
			Realm:  "baz",
			Status: Basic,
			Credentials: []string{
				"dXNlcjpwYXNzd29yZDE=",
				"bmFtZTpzZWNyZXQ=",
			},
			Condition: Condition{
				HostRegex: `^baz\.com$`,
			},
			UTF8: true,
		},
		{
			Realm:  "quux",
			Status: Proxy,
			Credentials: []string{
				"YmVudXR6ZXI6Z2VoZWlt",
				"QWxiZXJ0IEFkZGluOm9wZW4gc2V6IG1l",
			},
			Condition: Condition{
				URLRegex: "^/baz/quux",
			},
			UTF8: true,
		},
		{
			Realm:  "urlhost",
			Status: Basic,
			Credentials: []string{
				"dXJsOmhvc3Q=",
				"YWRtaW46c3VwZXJwb3dlcnM=",
			},
			Condition: Condition{
				HostRegex: `^url\.regex\.org$`,
				URLRegex:  "^/secret/path",
			},
		},
	},
}

func TestAuthTemplate(t *testing.T) {
	var buf bytes.Buffer
	gold := "auth.golden"
	tmplName := "auth.tmpl"

	tmpl, err := template.New(tmplName).Funcs(fMap).ParseFiles(tmplName)
	if err != nil {
		t.Error("Cannot parse auth template:", err)
		return
	}
	if err := tmpl.Execute(&buf, auths); err != nil {
		t.Error("auths template Execute():", err)
		return
	}
	ok, err := cmpGold(buf.Bytes(), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL for authorization does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", buf.String())
		}
	}
}
