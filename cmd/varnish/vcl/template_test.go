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
	"testing"
)

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

func TestTemplate(t *testing.T) {
	var buf bytes.Buffer
	if err := Tmpl.Execute(&buf, cafeSpec); err != nil {
		t.Error("Execute():", err)
	}
	t.Log(string(buf.Bytes()))
}
