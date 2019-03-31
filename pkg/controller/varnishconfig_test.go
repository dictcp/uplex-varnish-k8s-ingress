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

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
)

func TestValidateReqDisps(t *testing.T) {
	zero := int64(0)
	badDispSlice := [][]vcr_v1alpha1.RequestDispSpec{
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvSynth,
			},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.url",
				Compare:   vcr_v1alpha1.Equal,
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.url",
				Compare:   vcr_v1alpha1.Equal,
				Count:     &zero,
				Values:    []string{"/foo"},
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.url",
				Compare:   vcr_v1alpha1.NotExists,
				Count:     &zero,
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.url",
				Compare:   vcr_v1alpha1.LessEqual,
				Values:    []string{"/foo"},
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.url",
				Compare:   vcr_v1alpha1.Equal,
				Count:     &zero,
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.http.Host",
				Compare:   vcr_v1alpha1.Equal,
				Count:     &zero,
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.restarts",
				Compare:   vcr_v1alpha1.Equal,
				Values:    []string{"/foo"},
			}},
		}},
	}

	goodDispSlice := [][]vcr_v1alpha1.RequestDispSpec{
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.http.Host",
				Compare:   vcr_v1alpha1.NotExists,
			}},
		}},
		{{
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
			Conditions: []vcr_v1alpha1.ReqCondition{{
				Comparand: "req.http.Cookie",
				Compare:   vcr_v1alpha1.Exists,
			}},
		}},
	}

	for _, disps := range badDispSlice {
		if err := validateReqDisps(disps); err == nil {
			t.Errorf("validateReqDisps(%+v) expected error got=nil",
				disps)
		} else if testing.Verbose() {
			t.Logf("validateReqDisps(%+v) returned as expected: %v",
				disps, err)
		}
	}

	for _, disps := range goodDispSlice {
		if err := validateReqDisps(disps); err != nil {
			t.Errorf("validateReqDisps(%+v) expected no error "+
				"got='%+v'", disps, err)
		}
	}
}
