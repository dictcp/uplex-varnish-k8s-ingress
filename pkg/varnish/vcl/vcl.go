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

// Package vcl encapsulates representations of a VCL configuration
// derived from Ingress and VarnishConfig specifications, and
// checking the representations for equivalence (to check if new
// syncs are necessary). It drives the templating that generates
// VCL source code.
package vcl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"path"
	"regexp"
	"sort"
	"text/template"
)

// Address represents an endpoint for either a backend instance
// (Endpoint of a Service to which requests are routed) or a Varnish
// instance (where the port is the admin port).
type Address struct {
	IP   string
	Port int32
}

func (addr Address) hash(hash hash.Hash) {
	portBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(portBytes, uint32(addr.Port))
	hash.Write([]byte(addr.IP))
	hash.Write(portBytes)
}

// interface for sorting []Address
type byIPPort []Address

func (a byIPPort) Len() int      { return len(a) }
func (a byIPPort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byIPPort) Less(i, j int) bool {
	if a[i].IP < a[j].IP {
		return true
	}
	return a[i].Port < a[j].Port
}

// Service represents either a backend Service (Endpoints to which
// requests are routed) or a Varnish Service (with addresses for the
// admin ports).
type Service struct {
	Name      string
	Addresses []Address
}

func (svc Service) hash(hash hash.Hash) {
	hash.Write([]byte(svc.Name))
	for _, addr := range svc.Addresses {
		addr.hash(hash)
	}
}

// interface for sorting []Service
type byName []Service

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// Rule represents an IngressRule: a Host name (possibly empty) and a
// map from URL paths to Services.
type Rule struct {
	Host    string
	PathMap map[string]Service
}

func (rule Rule) hash(hash hash.Hash) {
	hash.Write([]byte(rule.Host))
	paths := make([]string, len(rule.PathMap))
	i := 0
	for k := range rule.PathMap {
		paths[i] = k
		i++
	}
	sort.Strings(paths)
	for _, p := range paths {
		hash.Write([]byte(p))
		rule.PathMap[p].hash(hash)
	}
}

// interface for sorting []Rule
type byHost []Rule

func (a byHost) Len() int           { return len(a) }
func (a byHost) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byHost) Less(i, j int) bool { return a[i].Host < a[j].Host }

// Probe represents the configuration of health probes derived from
// the VarnishConfig Custom Resource.
type Probe struct {
	Timeout   string
	Interval  string
	Initial   string
	Window    string
	Threshold string
}

func (probe Probe) hash(hash hash.Hash) {
	hash.Write([]byte(probe.Timeout))
	hash.Write([]byte(probe.Interval))
	hash.Write([]byte(probe.Initial))
	hash.Write([]byte(probe.Window))
	hash.Write([]byte(probe.Threshold))
}

// ShardCluster represents the configuration for self-sharding derived
// from the VarnishConfig Custom Resource.
type ShardCluster struct {
	Nodes           []Service
	Probe           Probe
	MaxSecondaryTTL string
}

func (shard ShardCluster) hash(hash hash.Hash) {
	for _, node := range shard.Nodes {
		node.hash(hash)
	}
	shard.Probe.hash(hash)
	hash.Write([]byte(shard.MaxSecondaryTTL))
}

// Condition specifies conditions under which an authentication
// protocols must be executed -- the URL path or the Host must match
// patterns, the request must be received from a TLS offloader, or any
// combination of the three.
type Condition struct {
	URLRegex  string
	HostRegex string
	TLS       bool
}

// AuthStatus is the response code to be sent for authentication
// failures, and serves to distinguish the protocols.
type AuthStatus uint16

const (
	// Basic Authentication
	Basic AuthStatus = 401
	// Proxy Authentication
	Proxy = 407
)

// Auth specifies Basic or Proxy Authentication, derived from an
// AuthSpec in a VarnishConfig resource.
type Auth struct {
	Realm       string
	Credentials []string
	Status      AuthStatus
	Condition   Condition
	UTF8        bool
}

func (auth Auth) hash(hash hash.Hash) {
	hash.Write([]byte(auth.Realm))
	for _, cred := range auth.Credentials {
		hash.Write([]byte(cred))
	}
	statusBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(statusBytes, uint16(auth.Status))
	hash.Write(statusBytes)
	hash.Write([]byte(auth.Condition.URLRegex))
	hash.Write([]byte(auth.Condition.HostRegex))
	if auth.Condition.TLS {
		hash.Write([]byte("TLS"))
	}
	if auth.UTF8 {
		hash.Write([]byte("UTF8"))
	}
}

// interface for sorting []Auth
type byRealm []Auth

func (a byRealm) Len() int           { return len(a) }
func (a byRealm) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byRealm) Less(i, j int) bool { return a[i].Realm < a[j].Realm }

// Spec is the specification for a VCL configuration derived from
// Ingresses and VarnishConfig Custom Resources. This abstracts the
// VCL to be loaded by all instances of a Varnish Service.
type Spec struct {
	// DefaultService corresponds to the default IngressBackend in
	// an Ingress, if present.
	DefaultService Service
	// Rules corresponds to the IngressRules in an Ingress.
	Rules []Rule
	// AllServices is a map of Service names to Service
	// configurations for all IngressBackends mentioned in an
	// Ingress, including the default Backend, and all Backends to
	// which requests are to be routed.
	AllServices map[string]Service
	// ShardCluster is derived from the self-sharding
	// specification in a VarnishConfig resource.
	ShardCluster ShardCluster
	// Auths is a list of specifications for Basic or Proxy
	// Authentication, derived from the Auth section of a
	// VarnishConfig.
	Auths []Auth
}

// DeepHash computes a 64-bit hash value from a Spec such that if two
// Specs are deeply equal, then their hash values are equal.
func (spec Spec) DeepHash() uint64 {
	hash := fnv.New64a()
	spec.DefaultService.hash(hash)
	for _, rule := range spec.Rules {
		rule.hash(hash)
	}
	svcs := make([]string, len(spec.AllServices))
	i := 0
	for k := range spec.AllServices {
		svcs[i] = k
		i++
	}
	sort.Strings(svcs)
	for _, svc := range svcs {
		hash.Write([]byte(svc))
		spec.AllServices[svc].hash(hash)
	}
	spec.ShardCluster.hash(hash)
	for _, auth := range spec.Auths {
		auth.hash(hash)
	}
	return hash.Sum64()
}

// Canonical returns a canonical form of a Spec, in which all of its
// fields are ordered. This ensures that reflect.DeepEqual and
// DeepHash return values consistent with the equivalence of two
// Specs.
func (spec Spec) Canonical() Spec {
	canon := Spec{
		DefaultService: Service{Name: spec.DefaultService.Name},
		Rules:          make([]Rule, len(spec.Rules)),
		AllServices:    make(map[string]Service, len(spec.AllServices)),
		ShardCluster:   spec.ShardCluster,
		Auths:          make([]Auth, len(spec.Auths)),
	}
	copy(canon.DefaultService.Addresses, spec.DefaultService.Addresses)
	sort.Stable(byIPPort(canon.DefaultService.Addresses))
	copy(canon.Rules, spec.Rules)
	sort.Stable(byHost(canon.Rules))
	for _, rule := range canon.Rules {
		for _, svc := range rule.PathMap {
			sort.Stable(byIPPort(svc.Addresses))
		}
	}
	for name, svcs := range spec.AllServices {
		canon.AllServices[name] = svcs
		sort.Stable(byIPPort(canon.AllServices[name].Addresses))
	}
	sort.Stable(byName(canon.ShardCluster.Nodes))
	for _, node := range canon.ShardCluster.Nodes {
		sort.Stable(byIPPort(node.Addresses))
	}
	copy(canon.Auths, spec.Auths)
	sort.Stable(byRealm(canon.Auths))
	for _, auth := range canon.Auths {
		sort.Strings(auth.Credentials)
	}
	return canon
}

var fMap = template.FuncMap{
	"plusOne":   func(i int) int { return i + 1 },
	"vclMangle": func(s string) string { return mangle(s) },
	"backendName": func(svc Service, addr string) string {
		return backendName(svc, addr)
	},
	"dirName": func(svc Service) string {
		return directorName(svc)
	},
	"urlMatcher": func(rule Rule) string {
		return urlMatcher(rule)
	},
}

const (
	ingTmplSrc   = "vcl.tmpl"
	shardTmplSrc = "self-shard.tmpl"
	authTmplSrc  = "auth.tmpl"
)

var (
	ingressTmpl *template.Template
	shardTmpl   *template.Template
	authTmpl    *template.Template
	symPattern  = regexp.MustCompile("^[[:alpha:]][[:word:]-]*$")
	first       = regexp.MustCompile("[[:alpha:]]")
	restIllegal = regexp.MustCompile("[^[:word:]-]+")
)

// InitTemplates initializes templates for VCL generation.
func InitTemplates(tmplDir string) error {
	var err error
	ingTmplPath := path.Join(tmplDir, ingTmplSrc)
	shardTmplPath := path.Join(tmplDir, shardTmplSrc)
	authTmplPath := path.Join(tmplDir, authTmplSrc)
	ingressTmpl, err = template.New(ingTmplSrc).
		Funcs(fMap).ParseFiles(ingTmplPath)
	if err != nil {
		return err
	}
	shardTmpl, err = template.New(shardTmplSrc).
		Funcs(fMap).ParseFiles(shardTmplPath)
	if err != nil {
		return err
	}
	authTmpl, err = template.New(authTmplSrc).
		Funcs(fMap).ParseFiles(authTmplPath)
	if err != nil {
		return err
	}
	return nil
}

func replIllegal(ill []byte) []byte {
	repl := []byte("_")
	for _, b := range ill {
		repl = append(repl, []byte(fmt.Sprintf("%02x", b))...)
	}
	repl = append(repl, []byte("_")...)
	return repl
}

// GetSrc returns the VCL generated to implement a Spec.
func (spec Spec) GetSrc() (string, error) {
	var buf bytes.Buffer
	if err := ingressTmpl.Execute(&buf, spec); err != nil {
		return "", err
	}
	if len(spec.ShardCluster.Nodes) > 0 {
		if err := shardTmpl.Execute(&buf, spec.ShardCluster); err != nil {
			return "", err
		}
	}
	if len(spec.Auths) > 0 {
		if err := authTmpl.Execute(&buf, spec); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func mangle(s string) string {
	var mangled string
	bytes := []byte(s)
	if s == "" || symPattern.Match(bytes) {
		return s
	}
	mangled = string(bytes[0])
	if !first.Match(bytes[0:1]) {
		mangled = "V" + mangled
	}
	rest := restIllegal.ReplaceAllFunc(bytes[1:], replIllegal)
	mangled = mangled + string(rest)
	return mangled
}

func backendName(svc Service, addr string) string {
	return mangle(svc.Name + "_" + addr)
}

func directorName(svc Service) string {
	return mangle(svc.Name + "_director")
}

func urlMatcher(rule Rule) string {
	return mangle(rule.Host + "_url")
}
