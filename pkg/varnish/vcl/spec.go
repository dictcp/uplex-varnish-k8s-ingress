/*
 * Copyright (c) 2019 UPLEX Nils Goroll Systemoptimierung
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
	"encoding/binary"
	"hash"
	"hash/fnv"
	"math"
	"sort"
)

func hashUint16(u16 uint16, hash hash.Hash) {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, u16)
	hash.Write(bytes)
}

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

// DirectorType corresponds to a class of director, see:
// https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html
type DirectorType uint8

const (
	// RoundRobin director
	RoundRobin DirectorType = iota
	// Random director
	Random
	// Shard director
	Shard
)

func (dirType DirectorType) String() string {
	switch dirType {
	case RoundRobin:
		return "round_robin"
	case Random:
		return "random"
	case Shard:
		return "shard"
	default:
		return "__INVALID_DIRECTOR_TYPE__"
	}
}

// GetDirectorType returns a DirectorType constant for the string
// (enum value) used in YAML.
func GetDirectorType(dirStr string) DirectorType {
	switch dirStr {
	case "round-robin":
		return RoundRobin
	case "random":
		return Random
	case "shard":
		return Shard
	default:
		return DirectorType(255)
	}
}

// Director is derived from spec.director in a BackendConfig, and allows
// for some choice of the director, and sets some parameters.
type Director struct {
	Rampup string
	Warmup float64
	Type   DirectorType
}

func (dir Director) hash(hash hash.Hash) {
	hash.Write([]byte(dir.Rampup))
	w64 := math.Float64bits(dir.Warmup)
	wBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(wBytes, w64)
	hash.Write(wBytes)
	hash.Write([]byte{byte(dir.Type)})
}

// Service represents either a backend Service (Endpoints to which
// requests are routed) or a Varnish Service (with addresses for the
// admin ports).
type Service struct {
	Name                string
	Addresses           []Address
	Probe               *Probe
	Director            *Director
	HostHeader          string
	ConnectTimeout      string
	FirstByteTimeout    string
	BetweenBytesTimeout string
	MaxConnections      uint32
	ProxyHeader         uint8
}

func (svc Service) hash(hash hash.Hash) {
	hash.Write([]byte(svc.Name))
	for _, addr := range svc.Addresses {
		addr.hash(hash)
	}
	if svc.Probe != nil {
		svc.Probe.hash(hash)
	}
	if svc.Director != nil {
		svc.Director.hash(hash)
	}
	hash.Write([]byte(svc.HostHeader))
	hash.Write([]byte(svc.ConnectTimeout))
	hash.Write([]byte(svc.FirstByteTimeout))
	hash.Write([]byte(svc.BetweenBytesTimeout))
	hash.Write([]byte{byte(svc.ProxyHeader)})
	maxConnBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(maxConnBytes, svc.MaxConnections)
	hash.Write(maxConnBytes)
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
// a VarnishConfig or BackendConfig Custom Resource.
type Probe struct {
	URL         string
	Request     []string
	ExpResponse uint16
	Timeout     string
	Interval    string
	Initial     string
	Window      string
	Threshold   string
}

func (probe Probe) hash(hash hash.Hash) {
	hash.Write([]byte(probe.URL))
	for _, r := range probe.Request {
		hash.Write([]byte(r))
	}
	hashUint16(probe.ExpResponse, hash)
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

// CompareType classifies comparisons for MatchTerm.
type CompareType uint8

const (
	// Equal means compare strings for equality (==).
	Equal CompareType = iota
	// NotEqual means compare with !=.
	NotEqual
	// Match means compare for regex match (~) -- the MatchTerm
	// Value is a regular expression.
	Match
	// NotMatch means compare with !~.
	NotMatch
)

// MatchTerm is a term describing the comparison of a VCL object with
// a pattern.
type MatchTerm struct {
	Comparand string
	Value     string
	Compare   CompareType
}

func (match MatchTerm) hash(hash hash.Hash) {
	hash.Write([]byte(match.Comparand))
	hash.Write([]byte(match.Value))
	hash.Write([]byte{byte(match.Compare)})
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
	// VCL is custom VCL, derived from VarnishConfig.Spec.VCL.
	VCL string
	// ACLs is a list of specifications for whitelisting or
	// blacklisting IPs with access control lists, derived from
	// VarnishConfig.Spec.ACLs.
	ACLs []ACL
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
	hash.Write([]byte(spec.VCL))
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
		VCL:            spec.VCL,
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
