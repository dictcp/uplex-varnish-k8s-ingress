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

package controller

import (
	"testing"

	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ing1 = &extensions.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "ing1",
	},
	Spec: extensions.IngressSpec{
		Backend: &extensions.IngressBackend{
			ServiceName: "default-svc2",
		},
		Rules: []extensions.IngressRule{
			extensions.IngressRule{Host: "host1"},
		},
	},
}

var ing2 = &extensions.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "ing2",
	},
	Spec: extensions.IngressSpec{
		Backend: &extensions.IngressBackend{
			ServiceName: "default-svc2",
		},
		Rules: []extensions.IngressRule{
			extensions.IngressRule{Host: "host2"},
		},
	},
}

var ing3 = &extensions.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "ing3",
	},
	Spec: extensions.IngressSpec{
		Rules: []extensions.IngressRule{
			extensions.IngressRule{Host: "host1"},
			extensions.IngressRule{Host: "host2"},
		},
	},
}

func TestIngressMergeError(t *testing.T) {
	ings := []*extensions.Ingress{ing1, ing2}
	if err := ingMergeError(ings); err == nil {
		t.Errorf("ingMergeError(): no error reported for more than " +
			"one default backend")
	} else if testing.Verbose() {
		t.Logf("ingMergeError() returned as expected: %v", err)
	}

	ings = []*extensions.Ingress{ing2, ing3}
	if err := ingMergeError(ings); err == nil {
		t.Errorf("ingMergeError(): no error reported for overlapping " +
			"Hosts")
	} else if testing.Verbose() {
		t.Logf("ingMergeError() returned as expected: %v", err)
	}
}
