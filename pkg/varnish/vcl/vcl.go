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
	"strings"
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

// NoMaskBits is a sentinel value for ACLAddress.MaskBits indicating
// that a CIDR range is not to be used.
const NoMaskBits uint8 = 255

// ACLAddress represents an element in an ACL -- a host name to be
// resolved at VCL load, an IP address, or address range in CIDR
// notation. Use the '!' for negation when Negate is true.
type ACLAddress struct {
	Addr     string
	MaskBits uint8
	Negate   bool
}

func (addr ACLAddress) hash(hash hash.Hash) {
	hash.Write([]byte(addr.Addr))
	hash.Write([]byte{byte(addr.MaskBits)})
	if addr.Negate {
		hash.Write([]byte("Negate"))
	}
}

// interface for sorting []ACLAddress
type byACLAddr []ACLAddress

func (a byACLAddr) Len() int           { return len(a) }
func (a byACLAddr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byACLAddr) Less(i, j int) bool { return a[i].Addr < a[j].Addr }

// MatchTerm is a term describing the comparison of a VCL object with
// a pattern.
type MatchTerm struct {
	Comparand string
	Regex     string
	Match     bool
}

func (match MatchTerm) hash(hash hash.Hash) {
	hash.Write([]byte(match.Comparand))
	hash.Write([]byte(match.Regex))
	if match.Match {
		hash.Write([]byte("Match"))
	}
}

// interface for sorting []MatchTerm
type byComparand []MatchTerm

func (a byComparand) Len() int      { return len(a) }
func (a byComparand) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byComparand) Less(i, j int) bool {
	return a[i].Comparand < a[j].Comparand
}

// ACL represents an Access Control List, derived from a
// VarnishConfig.
type ACL struct {
	Name       string
	Comparand  string
	FailStatus uint16
	Whitelist  bool
	Addresses  []ACLAddress
	Conditions []MatchTerm
}

func (acl ACL) hash(hash hash.Hash) {
	hash.Write([]byte(acl.Name))
	hash.Write([]byte(acl.Comparand))
	statusBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(statusBytes, uint16(acl.FailStatus))
	hash.Write(statusBytes)
	for _, addr := range acl.Addresses {
		addr.hash(hash)
	}
	for _, cond := range acl.Conditions {
		cond.hash(hash)
	}
}

// interface for sorting []ACL
type byACLName []ACL

func (a byACLName) Len() int           { return len(a) }
func (a byACLName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byACLName) Less(i, j int) bool { return a[i].Name < a[j].Name }

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
	ACLs  []ACL
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
	for _, acl := range spec.ACLs {
		acl.hash(hash)
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
		ACLs:           make([]ACL, len(spec.ACLs)),
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
	copy(canon.ACLs, spec.ACLs)
	sort.Stable(byACLName(canon.ACLs))
	for _, acl := range canon.ACLs {
		sort.Stable(byACLAddr(acl.Addresses))
		sort.Stable(byComparand(acl.Conditions))
	}
	return canon
}

var fMap = template.FuncMap{
	"plusOne":   func(i int) int { return i + 1 },
	"vclMangle": func(s string) string { return mangle(s) },
	"aclMask":   func(bits uint8) string { return aclMask(bits) },
	"aclCmp":    func(comparand string) string { return aclCmp(comparand) },
	"hasXFF":    func(acls []ACL) bool { return hasXFF(acls) },
	"backendName": func(svc Service, addr string) string {
		return backendName(svc, addr)
	},
	"dirName": func(svc Service) string {
		return directorName(svc)
	},
	"urlMatcher": func(rule Rule) string {
		return urlMatcher(rule)
	},
	"aclName": func(name string) string {
		return "vk8s_" + mangle(name) + "_acl"
	},
}

const (
	ingTmplSrc   = "vcl.tmpl"
	shardTmplSrc = "self-shard.tmpl"
	authTmplSrc  = "auth.tmpl"
	aclTmplSrc   = "acl.tmpl"
)

var (
	ingressTmpl *template.Template
	shardTmpl   *template.Template
	authTmpl    *template.Template
	aclTmpl     *template.Template
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
	aclTmplPath := path.Join(tmplDir, aclTmplSrc)

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
	aclTmpl, err = template.New(aclTmplSrc).
		Funcs(fMap).ParseFiles(aclTmplPath)
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
	if len(spec.ACLs) > 0 {
		if err := aclTmpl.Execute(&buf, spec); err != nil {
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

func aclMask(bits uint8) string {
	if bits > 128 {
		return ""
	}
	return fmt.Sprintf("/%d", bits)
}

const (
	xffFirst   = `regsub(req.http.X-Forwarded-For,"^([^,\s]+).*","\1")`
	xff2ndLast = `regsub(req.http.X-Forwarded-For,"^.*?([[:xdigit:]:.]+)\s*,[^,]*$","\1")`
)

func aclCmp(comparand string) string {
	if strings.HasPrefix(comparand, "xff-") ||
		strings.HasPrefix(comparand, "req.http.") {

		if comparand == "xff-first" {
			comparand = xffFirst
		} else if comparand == "xff-2ndlast" {
			comparand = xff2ndLast
		}
		return fmt.Sprintf(`std.ip(%s, "0.0.0.0")`, comparand)
	}
	return comparand
}

func hasXFF(acls []ACL) bool {
	for _, acl := range acls {
		if strings.HasPrefix(acl.Comparand, "xff-") {
			return true
		}
	}
	return false
}
