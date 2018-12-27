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
	Timeout   string `json:"timeout,omitempty"`
	Interval  string `json:"interval,omitempty"`
	Initial   *int32 `json:"initial,omitempty"`
	Window    *int32 `json:"window,omitempty"`
	Threshold *int32 `json:"threshold,omitempty"`
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
