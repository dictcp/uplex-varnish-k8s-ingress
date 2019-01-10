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
}

// SelfShardSpec specifies self-sharding in a Varnish cluster.
// see: https://code.uplex.de/uplex-varnish/k8s-ingress/blob/master/docs/self-sharding.md
type SelfShardSpec struct {
	Max2ndTTL string    `json:"max-secondary-ttl,omitempty"`
	Probe     ProbeSpec `json:"probe,omitempty"`
}

// ProbeSpec specifies health probes in use for self-sharding.
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
	Realm      string         `json:"realm"`
	SecretName string         `json:"secretName"`
	Type       AuthType       `json:"type,omitempty"`
	Condition  *AuthCondition `json:"condition,omitempty"`
	UTF8       bool           `json:"utf8,omitempty"`
}

// AuthType classifies the protocol for an AuthSpec.
type AuthType string

const (
	// Basic Authentication
	Basic AuthType = "basic"
	// Proxy Authentication
	Proxy = "proxy"
)

// AuthCondition specifies a condition under which an authentication
// protocol must be executed -- the URL path or the Host must match a
// pattern (or both).
type AuthCondition struct {
	URLRegex  string `json:"url-match,omitempty"`
	HostRegex string `json:"host-match,omitempty"`
}

// ACLSpec specifies whitelisting or blacklisting IP addresses against
// an access control list.
type ACLSpec struct {
	Name       string       `json:"name,omitempty"`
	ACLType    ACLType      `json:"type,omitempty"`
	Comparand  string       `json:"comparand,omitempty"`
	FailStatus *int32       `json:"fail-status,omitempty"`
	Addresses  []ACLAddress `json:"addrs,omitempty"`
	Conditions []Condition  `json:"conditions,omitempty"`
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
