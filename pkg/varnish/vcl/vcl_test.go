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

var acls = Spec{
	ACLs: []ACL{
		{
			Name:       "man_vcl_example",
			Comparand:  "client.ip",
			FailStatus: 403,
			Whitelist:  true,
			Addresses: []ACLAddress{
				ACLAddress{
					Addr:     "localhost",
					MaskBits: 255,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "192.0.2.0",
					MaskBits: 24,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "192.0.2.23",
					MaskBits: 255,
					Negate:   true,
				},
			},
			Conditions: []MatchTerm{
				MatchTerm{
					Comparand: "req.http.Host",
					Compare:   Equal,
					Value:     "cafe.example.com",
				},
				MatchTerm{
					Comparand: "req.url",
					Compare:   Match,
					Value:     `^/coffee(/|$)`,
				},
			},
		},
		{
			Name:       "wikipedia_example",
			Comparand:  "server.ip",
			FailStatus: 404,
			Whitelist:  false,
			Addresses: []ACLAddress{
				ACLAddress{
					Addr:     "192.168.100.14",
					MaskBits: 24,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "192.168.100.0",
					MaskBits: 22,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "2001:db8::",
					MaskBits: 48,
					Negate:   false,
				},
			},
			Conditions: []MatchTerm{
				MatchTerm{
					Comparand: "req.http.Host",
					Compare:   NotEqual,
					Value:     "cafe.example.com",
				},
				MatchTerm{
					Comparand: "req.url",
					Compare:   NotMatch,
					Value:     `^/tea(/|$)`,
				},
			},
		},
		{
			Name:       "private4",
			Comparand:  "req.http.X-Real-IP",
			FailStatus: 403,
			Whitelist:  true,
			Addresses: []ACLAddress{
				ACLAddress{
					Addr:     "10.0.0.0",
					MaskBits: 24,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "172.16.0.0",
					MaskBits: 12,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "192.168.0.0",
					MaskBits: 16,
					Negate:   false,
				},
			},
			Conditions: []MatchTerm{},
		},
		{
			Name:       "rfc5737",
			Comparand:  "xff-first",
			FailStatus: 403,
			Whitelist:  true,
			Addresses: []ACLAddress{
				ACLAddress{
					Addr:     "192.0.2.0",
					MaskBits: 24,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "198.51.100.0",
					MaskBits: 24,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "203.0.113.0",
					MaskBits: 24,
					Negate:   false,
				},
			},
			Conditions: []MatchTerm{},
		},
		{
			Name:       "local",
			Comparand:  "xff-2ndlast",
			FailStatus: 403,
			Whitelist:  true,
			Addresses: []ACLAddress{
				ACLAddress{
					Addr:     "127.0.0.0",
					MaskBits: 8,
					Negate:   false,
				},
				ACLAddress{
					Addr:     "::1",
					MaskBits: 255,
					Negate:   false,
				},
			},
			Conditions: []MatchTerm{},
		},
	},
}

func TestAclTemplate(t *testing.T) {
	var buf bytes.Buffer
	gold := "acl.golden"
	tmplName := "acl.tmpl"

	tmpl, err := template.New(tmplName).Funcs(fMap).ParseFiles(tmplName)
	if err != nil {
		t.Error("Cannot parse acl template:", err)
		return
	}
	if err := tmpl.Execute(&buf, acls); err != nil {
		t.Error("acls template Execute():", err)
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

var customVCLSpec = Spec{
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
	VCL: `sub vcl_deliver {
	set resp.http.Hello = "world";
}`,
}

func TestCustomVCL(t *testing.T) {
	gold := "custom_vcl.golden"

	vcl, err := customVCLSpec.GetSrc()
	if err != nil {
		t.Fatal("GetSrc():", err)
	}

	ok, err := cmpGold([]byte(vcl), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL for custom VCL does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Log("Generated: ", vcl)
		}
	}
}

var teaSvcProbeDir = Service{
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
	HostHeader:          "tea.svc.org",
	ConnectTimeout:      "1s",
	FirstByteTimeout:    "2s",
	BetweenBytesTimeout: "2s",
	MaxConnections:      200,
	ProxyHeader:         1,
	Probe: &Probe{
		URL:         "/healthz",
		ExpResponse: 204,
		Timeout:     "5s",
		Interval:    "5s",
		Initial:     "2",
		Window:      "8",
		Threshold:   "3",
	},
	Director: &Director{
		Type:   Shard,
		Rampup: "5m",
		Warmup: 0.5,
	},
}

var coffeeSvcProbeDir = Service{
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
	HostHeader:          "coffee.svc.org",
	ConnectTimeout:      "3s",
	FirstByteTimeout:    "2s",
	BetweenBytesTimeout: "1s",
	ProxyHeader:         2,
	Probe: &Probe{
		Request: []string{
			"GET /healthz HTTP/1.1",
			"Host: coffee.svc.org",
			"Connection: close",
		},
		Timeout:   "4s",
		Interval:  "4s",
		Initial:   "1",
		Window:    "7",
		Threshold: "2",
	},
	Director: &Director{
		Type: Random,
	},
}

var milkSvcProbeDir = Service{
	Name: "milk-svc",
	Addresses: []Address{
		{
			IP:   "192.0.2.6",
			Port: 80,
		},
		{
			IP:   "192.0.2.7",
			Port: 80,
		},
	},
	HostHeader:       "milk.svc.org",
	FirstByteTimeout: "3s",
	Probe: &Probe{
		Timeout:   "5s",
		Interval:  "5s",
		Window:    "3",
		Threshold: "2",
	},
	Director: &Director{
		Type: RoundRobin,
	},
}

var cafeProbeDir = Spec{
	DefaultService: Service{},
	Rules: []Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]Service{
			"/tea":    teaSvcProbeDir,
			"/coffee": coffeeSvcProbeDir,
			"/milk":   milkSvcProbeDir,
		},
	}},
	AllServices: map[string]Service{
		"tea-svc":    teaSvcProbeDir,
		"coffee-svc": coffeeSvcProbeDir,
		"milk-svc":   milkSvcProbeDir,
	},
}

func TestBackendConfig(t *testing.T) {
	var buf bytes.Buffer
	gold := "backendcfg.golden"

	if err := ingressTmpl.Execute(&buf, cafeProbeDir); err != nil {
		t.Fatal("Execute():", err)
	}

	ok, err := cmpGold(buf.Bytes(), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL for BackendConfig does not match gold "+
			"file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", buf.String())
		}
	}
}
