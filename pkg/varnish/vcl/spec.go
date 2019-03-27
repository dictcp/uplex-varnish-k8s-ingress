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
	"crypto/sha512"
	"encoding/binary"
	"hash"
	"math"
	"math/big"
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
	Conditions  []MatchTerm
	Credentials []string
	Realm       string
	Status      AuthStatus
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
	for _, cond := range auth.Conditions {
		cond.hash(hash)
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

// ResultHdrType describes the configuration for writing a header as
// the result of an ACL comparison. Header is the header to write in
// VCL notation. Failure is the value to write on the fail condition,
// Success the value to write otherwise.
type ResultHdrType struct {
	Header  string
	Success string
	Failure string
}

func (resultHdr ResultHdrType) hash(hash hash.Hash) {
	hash.Write([]byte(resultHdr.Header))
	hash.Write([]byte(resultHdr.Success))
	hash.Write([]byte(resultHdr.Failure))
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
	ResultHdr  ResultHdrType
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
	acl.ResultHdr.hash(hash)
}

// interface for sorting []ACL
type byACLName []ACL

func (a byACLName) Len() int           { return len(a) }
func (a byACLName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byACLName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// RewriteRule describes conditions under which a rewrite is executed,
// and strings to be used for the rewrite. Elements of the Rules array
// for a Rewrite have this type.
//
// Value is a string or pattern against which the Source object is
// compared. If Values is a regular expression, it has the syntax and
// semantics of RE2 (https://github.com/google/re2/wiki/Syntax).
//
// Rewrite is a string to be used for the rewrite if the Value
// compares successfully. Depending on the RewriteSpec's Method,
// Rewrite may contain backreferences captured from a regex match.
type RewriteRule struct {
	Value   string
	Rewrite string
}

// MethodType classifies the process by which a rewrite modifies the
// Target object.
type MethodType uint8

const (
	// Replace means that the target is overwritten with a new
	// value.
	Replace MethodType = iota
	// Sub means that the first matching substring of the target
	// after a regex match is substituted with the new value.
	Sub
	// Suball means that each non-overlapping matching substring
	// of the target is substituted.
	Suball
	// RewriteMethod means that the target is rewritten with the
	// rule in the Rewrite field, possibly with backreferences.
	RewriteMethod
	// Append means that a string is concatenated after the source
	// string, with the result written to the target.
	Append
	// Prepend means that a string is concatenated after the
	// source string.
	Prepend
	// Delete means that the target object is deleted.
	Delete
)

// RewriteCompare classifies the comparison operation used to evaluate
// the conditions for a rewrite.
type RewriteCompare uint8

const (
	// RewriteMatch means that a regex match is executed.
	RewriteMatch RewriteCompare = iota
	// RewriteEqual means that fixed strings are tested for equality.
	RewriteEqual
	// Prefix indicates a fixed-string prefix match.
	Prefix
)

// SubType classifies the VCL subroutine in which a rewrite is
// executed.
type SubType uint8

const (
	// Unspecified means that the VCL sub was not specified in the
	// user configuration, and will be inferred from the Source
	// and Target.
	Unspecified SubType = iota
	// Recv for vcl_recv
	Recv
	// Pipe for vcl_pipe
	Pipe
	// Pass for vcl_pass
	Pass
	// Hash for vcl_hash
	Hash
	// Purge for vcl_purge
	Purge
	// Miss for vcl_miss
	Miss
	// Hit for vcl_hit
	Hit
	// Deliver for vcl_deliver
	Deliver
	// Synth for vcl_synth
	Synth
	// BackendFetch for vcl_backend_fetch
	BackendFetch
	// BackendResponse for vcl_backend_response
	BackendResponse
	// BackendError for vcl_backend_error
	BackendError
)

// AnchorType classifies start-of-string and/or end-of-string
// anchoring for a regex match.
type AnchorType uint8

const (
	// None indicates no anchoring.
	None AnchorType = iota
	// Start indicates anchoring at start-of-string.
	Start
	// Both indicates anchoring at start- and end-of-string.
	Both
)

// MatchFlagsType is a collection of options that modify matching
// operations. CaseSensitive can be applied to both fixed string
// matches and regex matches; the remainder are for regex matches
// only. These correspond to flags for RE2 matching, and are used
// by the RE2 VMOD.
//
// See: https://code.uplex.de/uplex-varnish/libvmod-re2
type MatchFlagsType struct {
	MaxMem        uint64
	Anchor        AnchorType
	UTF8          bool
	PosixSyntax   bool
	LongestMatch  bool
	Literal       bool
	NeverCapture  bool
	CaseSensitive bool
	PerlClasses   bool
	WordBoundary  bool
}

func (flags MatchFlagsType) hash(hash hash.Hash) {
	memBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(memBytes, flags.MaxMem)
	hash.Write(memBytes)
	hash.Write([]byte{byte(flags.Anchor)})
	if flags.UTF8 {
		hash.Write([]byte("UTF8"))
	}
	if flags.PosixSyntax {
		hash.Write([]byte("PosixSyntax"))
	}
	if flags.LongestMatch {
		hash.Write([]byte("LongestMatch"))
	}
	if flags.Literal {
		hash.Write([]byte("Literal"))
	}
	if flags.NeverCapture {
		hash.Write([]byte("NeverCapture"))
	}
	if flags.CaseSensitive {
		hash.Write([]byte("CaseSensitive"))
	}
	if flags.PerlClasses {
		hash.Write([]byte("PerlClasses"))
	}
	if flags.WordBoundary {
		hash.Write([]byte("WordBoundary"))
	}
}

// SelectType classifies the determination of the rewrite rule to
// apply if more than one of them in the Rules array compares
// successfully. This is only possible when the Method specifies a
// regex or prefix match.
//
// The values Unique, First or Last may be used for both regex and
// prefix matches; the others may only be used for prefix matches.
type SelectType uint8

const (
	// Unique means that only one rewrite rule may match,
	// otherwise VCL failure is invoked.
	Unique SelectType = iota
	// First means that the first matching rule in the order of
	// the Rules array is executed.
	// Last means that the last matching rule is executed.
	First
	// Last means that the last matching rule is executed.
	Last
	// Exact means that, for a prefix match, the rule by which the
	// full string matched exactly is executed.
	Exact
	// Longest means that the rule for the longest prefix that
	// matched is executed.
	Longest
	// Shortest means that the rule for the shortest prefix that
	// matched is executed.
	Shortest
)

// The String method returns the upper-case name of the enum used for
// VMOD re2.
func (s SelectType) String() string {
	switch s {
	case Unique:
		return "UNIQUE"
	case First:
		return "FIRST"
	case Last:
		return "LAST"
	case Exact:
		return "EXACT"
	case Longest:
		return "LONGEST"
	case Shortest:
		return "SHORTEST"
	default:
		return "__INVALID_SELECT__"
	}
}

// Rewrite is the specification for a set of rewrite rules; elements
// of the Rewrites array of a VCL Spec have this type.
//
// Target is the object to be rewritten, in VCL notation. It can be
// the client or backend URL path, or a client or backend request or
// response header.
//
// Source is the object against which comparisons are applied, and
// from which substrings may be extracted. It may have the same values
// as Target.
type Rewrite struct {
	Rules      []RewriteRule
	MatchFlags MatchFlagsType
	Target     string
	Source     string
	Method     MethodType
	Compare    RewriteCompare
	VCLSub     SubType
	Select     SelectType
}

func (rw Rewrite) hash(hash hash.Hash) {
	for _, rule := range rw.Rules {
		hash.Write([]byte(rule.Value))
		hash.Write([]byte(rule.Rewrite))
	}
	rw.MatchFlags.hash(hash)
	hash.Write([]byte(rw.Target))
	hash.Write([]byte(rw.Source))
	hash.Write([]byte{byte(rw.Method)})
	hash.Write([]byte{byte(rw.Compare)})
	hash.Write([]byte{byte(rw.VCLSub)})
	hash.Write([]byte{byte(rw.Select)})
}

// interface for sorting []Rewrite
type byVCLSub []Rewrite

func (a byVCLSub) Len() int           { return len(a) }
func (a byVCLSub) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byVCLSub) Less(i, j int) bool { return a[i].VCLSub < a[j].VCLSub }

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
	ACLs     []ACL
	Rewrites []Rewrite
}

// DeepHash computes a alphanumerically encoded hash value from a Spec
// such that, almost certainly, two Specs are deeply equal iff their
// hash values are equal (unless we've discovered a SHA512 collision).
func (spec Spec) DeepHash() string {
	hash := sha512.New512_224()
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
	for _, rw := range spec.Rewrites {
		rw.hash(hash)
	}
	h := new(big.Int)
	h.SetBytes(hash.Sum(nil))
	return h.Text(62)
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
		Rewrites:       make([]Rewrite, len(spec.Rewrites)),
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
	copy(canon.Rewrites, spec.Rewrites)
	sort.Stable(byVCLSub(canon.Rewrites))
	return canon
}
