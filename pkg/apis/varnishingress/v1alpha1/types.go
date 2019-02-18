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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VarnishConfig is the client API for the VarnishConfig Custom
// Resource, which specifies additional configuration and features for
// Services running Varnish as an implementation of Ingress.
type VarnishConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VarnishConfigSpec `json:"spec"`
	//        Status VarnishConfigStatus `json:"status"`
}

// VarnishConfigSpec corresponds to the spec section of a
// VarnishConfig Custom Resource.
type VarnishConfigSpec struct {
	Services     []string       `json:"services,omitempty"`
	SelfSharding *SelfShardSpec `json:"self-sharding,omitempty"`
	VCL          string         `json:"vcl,omitempty"`
	Auth         []AuthSpec     `json:"auth,omitempty"`
	ACLs         []ACLSpec      `json:"acl,omitempty"`
	Rewrites     []RewriteSpec  `json:"rewrites,omitempty"`
}

// SelfShardSpec specifies self-sharding in a Varnish cluster.
// see: https://code.uplex.de/uplex-varnish/k8s-ingress/blob/master/docs/self-sharding.md
type SelfShardSpec struct {
	Max2ndTTL string    `json:"max-secondary-ttl,omitempty"`
	Probe     ProbeSpec `json:"probe,omitempty"`
}

// ProbeSpec specifies health probes for self-sharding and BackendConfig.
// see: https://code.uplex.de/uplex-varnish/k8s-ingress/blob/master/docs/self-sharding.md
type ProbeSpec struct {
	URL         string   `json:"url,omitempty"`
	Request     []string `json:"request,omitempty"`
	ExpResponse *int32   `json:"expected-response,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	Interval    string   `json:"interval,omitempty"`
	Initial     *int32   `json:"initial,omitempty"`
	Window      *int32   `json:"window,omitempty"`
	Threshold   *int32   `json:"threshold,omitempty"`
}

// AuthSpec specifies authentication (basic or proxy).
type AuthSpec struct {
	Realm      string      `json:"realm"`
	SecretName string      `json:"secretName"`
	Type       AuthType    `json:"type,omitempty"`
	UTF8       bool        `json:"utf8,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
}

// AuthType classifies the protocol for an AuthSpec.
type AuthType string

const (
	// Basic Authentication
	Basic AuthType = "basic"
	// Proxy Authentication
	Proxy = "proxy"
)

// ACLSpec specifies whitelisting or blacklisting IP addresses against
// an access control list.
type ACLSpec struct {
	Name       string         `json:"name,omitempty"`
	ACLType    ACLType        `json:"type,omitempty"`
	Comparand  string         `json:"comparand,omitempty"`
	ResultHdr  *ResultHdrType `json:"result-header,omitempty"`
	FailStatus *int32         `json:"fail-status,omitempty"`
	Addresses  []ACLAddress   `json:"addrs,omitempty"`
	Conditions []Condition    `json:"conditions,omitempty"`
}

// ACLAddress represents an entry in a VCL. If MaskBits is non-nil, it
// is a CIDR range. If Negate is true, use the '!' notation in the VCL
// ACL.
type ACLAddress struct {
	Address  string `json:"addr,omitempty"`
	MaskBits *int32 `json:"mask-bits,omitempty"`
	Negate   bool   `json:"negate,omitempty"`
}

// Condition represents a term in a boolean expression -- test the
// Comparand against Value for equality or regex match.
type Condition struct {
	Comparand string      `json:"comparand,omitempty"`
	Compare   CompareType `json:"compare,omitempty"`
	Value     string      `json:"value,omitempty"`
}

// ACLType classifies an ACL.
type ACLType string

const (
	// Whitelist means that the failure status is returned when an
	// IP address does not match an ACL.
	Whitelist ACLType = "whitelist"
	// Blacklist means that the failure status is returned when an
	// IP address does match an ACL.
	Blacklist = "blacklist"
)

// CompareType classifies a string comparison.
type CompareType string

const (
	// Equal means compare strings with ==.
	Equal CompareType = "equal"
	// NotEqual means compare with !=.
	NotEqual = "not-equal"
	// Match means compare with ~ (the Value is a regex).
	Match = "match"
	// NotMatch means compare with !~.
	NotMatch = "not-match"
)

// ResultHdrType describes the configuration for writing a header as
// the result of an ACL comparison. Header is the header to write in
// VCL notation. Failure is the value to write on the fail condition,
// Success the value to write otherwise.
type ResultHdrType struct {
	Header  string `json:"header"`
	Success string `json:"success"`
	Failure string `json:"failure"`
}

// RewriteRule describes conditions under which a rewrite is executed, and
// strings to be used for the rewrite. Elements of the Rules array for a
// RewriteSpec have this type.
//
// Value is a string or pattern against which the Source object is
// compared. If Values is a regular expression, it has the syntax and
// semantics of RE2 (https://github.com/google/re2/wiki/Syntax).
//
// Rewrite is a string to be used for the rewrite if the Value
// compares successfully. Depending on the RewriteSpec's Method,
// Rewrite may contain backreferences captured from a regex match.
type RewriteRule struct {
	Value   string `json:"value,omitempty"`
	Rewrite string `json:"rewrite,omitempty"`
}

// AnchorType classifies start-of-string and/or end-of-string
// anchoring for a regex match.
type AnchorType string

const (
	// None indicates no anchoring.
	None AnchorType = "none"
	// Start indicates anchoring at start-of-string.
	Start = "start"
	// Both indicates anchoring at start- and end-of-string.
	Both = "both"
)

// MatchFlagsType is a collection of options that modify matching
// operations. CaseSensitive can be applied to both fixed string
// matches and regex matches; the remainder are for regex matches
// only. These correspond to flags for RE2 matching, and are used
// by the RE2 VMOD.
//
// See: https://code.uplex.de/uplex-varnish/libvmod-re2
type MatchFlagsType struct {
	MaxMem        *uint64    `json:"max-mem,omitempty"`
	Anchor        AnchorType `json:"anchor,omitempty"`
	CaseSensitive *bool      `json:"case-sensitive,omitempty"`
	UTF8          bool       `json:"utf8,omitempty"`
	PosixSyntax   bool       `json:"posix-syntax,omitempty"`
	LongestMatch  bool       `json:"longest-match,omitempty"`
	Literal       bool       `json:"literal,omitempty"`
	NeverCapture  bool       `json:"never-capture,omitempty"`
	PerlClasses   bool       `json:"perl-classes,omitempty"`
	WordBoundary  bool       `json:"word-boundary,omitempty"`
}

// MethodType classifies the process by which a rewrite modifies the
// Target object.
type MethodType string

const (
	// Replace means that the target is overwritten with a new
	// value.
	Replace MethodType = "replace"
	// Sub means that the first matching substring of the target
	// after a regex match is substituted with the new value.
	Sub = "sub"
	// Suball means that each non-overlapping matching substring
	// of the target is substituted.
	Suball = "suball"
	// Rewrite means that the target is rewritten with the rule in
	// the Rewrite field, possibly with backreferences.
	Rewrite = "rewrite"
	// Append means that a string is concatenated after the source
	// string, with the result written to the target.
	Append = "append"
	// Prepend means that a string is concatenated after the
	// source string.
	Prepend = "prepend"
	// Delete means that the target object is deleted.
	Delete = "delete"
)

// RewriteCompare classifies the comparison operation used to evaluate
// the conditions for a rewrite.
type RewriteCompare string

const (
	// RewriteMatch means that a regex match is executed.
	RewriteMatch RewriteCompare = "match"
	// RewriteEqual means that fixed strings are tested for equality.
	RewriteEqual = "equal"
	// Prefix indicates a fixed-string prefix match.
	Prefix = "prefix"
)

// VCLSubType classifies the VCL subroutine in which a rewrite is
// executed.
type VCLSubType string

const (
	// Recv for vcl_recv
	Recv VCLSubType = "recv"
	// Pipe for vcl_pipe
	Pipe = "pipe"
	// Pass for vcl_pass
	Pass = "pass"
	// Hash for vcl_hash
	Hash = "hash"
	// Purge for vcl_purge
	Purge = "purge"
	// Miss for vcl_miss
	Miss = "miss"
	// Hit for vcl_hit
	Hit = "hit"
	// Deliver for vcl_deliver
	Deliver = "deliver"
	// Synth for vcl_synth
	Synth = "synth"
	// BackendFetch for vcl_backend_fetch
	BackendFetch = "backend_fetch"
	// BackendResponse for vcl_backend_response
	BackendResponse = "backend_response"
	// BackendError for vcl_backend_error
	BackendError = "backend_error"
)

// SelectType classifies the determination of the rewrite rule to
// apply if more than one of them in the Rules array compares
// successfully. This is only possible when the Method specifies a
// regex or prefix match.
//
// The values Unique, First or Last may be used for both regex and
// prefix matches; the others may only be used for prefix matches.
type SelectType string

const (
	// Unique means that only one rewrite rule may match,
	// otherwise VCL failure is invoked.
	Unique SelectType = "unique"
	// First means that the first matching rule in the order of
	// the Rules array is executed.
	First = "first"
	// Last means that the last matching rule is executed.
	Last = "last"
	// Exact means that, for a prefix match, the rule by which the
	// full string matched exactly is executed.
	Exact = "exact"
	// Longest means that the rule for the longest prefix that
	// matched is executed.
	Longest = "longest"
	// Shortest means that the rule for the shortest prefix that
	// matched is executed.
	Shortest = "shortest"
)

// RewriteSpec is the configuration for a set of rewrite rules;
// elements of the Rewrites array of a VarnishConfigSpec have this
// type.
//
// Target is the object to be rewritten, in VCL notation. It can be
// the client or backend URL path, or a client or backend request or
// response header.
//
// Source is the object against which comparisons are applied, and
// from which substrings may be extracted. It may have the same values
// as Target. If Source is the empty string, then the Source is the
// same as the Target, and the Target is rewritten in place.
type RewriteSpec struct {
	Rules      []RewriteRule   `json:"rules,omitempty"`
	MatchFlags *MatchFlagsType `json:"match-flags,omitempty"`
	Target     string          `json:"target,omitempty"`
	Source     string          `json:"source,omitempty"`
	Method     MethodType      `json:"method,omitempty"`
	Compare    RewriteCompare  `json:"compare,omitempty"`
	VCLSub     VCLSubType      `json:"vcl-sub,omitempty"`
	Select     SelectType      `json:"select,omitempty"`
}

// VarnishConfigStatus is the status for a VarnishConfig resource
// type VarnishConfigStatus struct {
//         AvailableReplicas int32 `json:"availableReplicas"`
// }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VarnishConfigList is a list of VarnishConfig Custom Resources.
type VarnishConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VarnishConfig `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackendConfig is the client API for the BackendConfig Custom
// Resource, which specifies properties of an Ingress/Varnish backend,
// realized as a k8s Service.
type BackendConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BackendConfigSpec `json:"spec"`
	//        Status BackendConfigStatus `json:"status"`
}

// BackendConfigSpec corresponds to the spec section of a
// BackendConfig Custom Resource.
type BackendConfigSpec struct {
	Services            []string      `json:"services,omitempty"`
	Probe               *ProbeSpec    `json:"probe,omitempty"`
	Director            *DirectorSpec `json:"director,omitempty"`
	HostHeader          string        `json:"host-header,omitempty"`
	ConnectTimeout      string        `json:"connect-timeout,omitempty"`
	FirstByteTimeout    string        `json:"first-byte-timeout,omitempty"`
	BetweenBytesTimeout string        `json:"between-bytes-timeout,omitempty"`
	MaxConnections      *int32        `json:"max-connections,omitempty"`
	ProxyHeader         *int32        `json:"proxy-header,omitempty"`
}

// DirectorType specfies the class of director to be used, see:
// https://varnish-cache.org/docs/6.1/reference/vmod_directors.generated.html
type DirectorType string

const (
	// RoundRobin director
	RoundRobin DirectorType = "round-robin"
	// Random director
	Random = "random"
	// Shard director
	Shard = "shard"
)

// DirectorSpec corresponds to spec.director in a BackendConfig, and
// allows for a choice of directors, and some parameters.
type DirectorSpec struct {
	Type   DirectorType `json:"type,omitempty"`
	Warmup *int32       `json:"warmup,omitempty"`
	Rampup string       `json:"rampup,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackendConfigList is a list of BackendConfig Custom Resources.
type BackendConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BackendConfig `json:"items"`
}
