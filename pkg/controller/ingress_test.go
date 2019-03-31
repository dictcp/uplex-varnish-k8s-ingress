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
	"io/ioutil"
	"testing"

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"

	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"
	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
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

func TestConfigMatchFlags(t *testing.T) {
	zero := uint64(0)
	mb80 := uint64(1024 * 1024 * 80)
	nope := false
	yep := true

	exps := []struct {
		specFlags vcr_v1alpha1.MatchFlagsType
		vclFlags  vcl.MatchFlagsType
	}{
		{
			specFlags: vcr_v1alpha1.MatchFlagsType{
				MaxMem:        nil,
				Anchor:        vcr_v1alpha1.None,
				CaseSensitive: nil,
				UTF8:          false,
				PosixSyntax:   false,
				LongestMatch:  false,
				Literal:       false,
				NeverCapture:  false,
				PerlClasses:   false,
				WordBoundary:  false,
			},
			vclFlags: vcl.MatchFlagsType{
				MaxMem:        0,
				Anchor:        vcl.None,
				CaseSensitive: true,
				UTF8:          false,
				PosixSyntax:   false,
				LongestMatch:  false,
				Literal:       false,
				NeverCapture:  false,
				PerlClasses:   false,
				WordBoundary:  false,
			},
		},
		{
			specFlags: vcr_v1alpha1.MatchFlagsType{
				MaxMem:        &zero,
				Anchor:        vcr_v1alpha1.Start,
				CaseSensitive: &nope,
				UTF8:          true,
				PosixSyntax:   true,
				LongestMatch:  true,
				Literal:       true,
				NeverCapture:  true,
				PerlClasses:   true,
				WordBoundary:  true,
			},
			vclFlags: vcl.MatchFlagsType{
				MaxMem:        0,
				Anchor:        vcl.Start,
				CaseSensitive: false,
				UTF8:          true,
				PosixSyntax:   true,
				LongestMatch:  true,
				Literal:       true,
				NeverCapture:  true,
				PerlClasses:   true,
				WordBoundary:  true,
			},
		},
		{
			specFlags: vcr_v1alpha1.MatchFlagsType{
				MaxMem:        &mb80,
				Anchor:        vcr_v1alpha1.Both,
				CaseSensitive: &yep,
				UTF8:          true,
				PosixSyntax:   false,
				LongestMatch:  true,
				Literal:       false,
				NeverCapture:  true,
				PerlClasses:   false,
				WordBoundary:  true,
			},
			vclFlags: vcl.MatchFlagsType{
				MaxMem:        mb80,
				Anchor:        vcl.Both,
				CaseSensitive: true,
				UTF8:          true,
				PosixSyntax:   false,
				LongestMatch:  true,
				Literal:       false,
				NeverCapture:  true,
				PerlClasses:   false,
				WordBoundary:  true,
			},
		},
	}

	for _, exp := range exps {
		got := configMatchFlags(exp.specFlags)
		if got != exp.vclFlags {
			t.Errorf("configMatchFlags(%+v) expected=%+v got=%+v",
				exp.specFlags, exp.vclFlags, got)
		}
	}
}

var status400 = int64(400)
var status403 = int64(403)
var status405 = int64(405)
var zero = int64(0)
var uintZero = uint(0)
var nope = false
var reqDispHarness = []struct {
	spec []vcr_v1alpha1.RequestDispSpec
	exp  vcl.Spec
}{{
	spec: []vcr_v1alpha1.RequestDispSpec{
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.Equal,
					Values:    []string{"PRI"},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvSynth,
				Status: &status405,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.http.Host",
					Compare:   vcr_v1alpha1.NotExists,
				},
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.esi_level",
					Count:     &zero,
				},
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.proto",
					Compare:   vcr_v1alpha1.Prefix,
					Values:    []string{"HTTP/1.1"},
					MatchFlags: &vcr_v1alpha1.MatchFlagsType{
						CaseSensitive: &nope,
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvSynth,
				Status: &status400,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.NotEqual,
					Values: []string{
						"GET",
						"HEAD",
						"PUT",
						"POST",
						"TRACE",
						"OPTIONS",
						"DELETE",
						"PATCH",
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPipe,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.NotEqual,
					Values: []string{
						"GET",
						"HEAD",
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.http.Cookie",
					Compare:   vcr_v1alpha1.Exists,
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.http.Authorization",
					Compare:   vcr_v1alpha1.Exists,
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
		},
	},
	exp: vcl.Spec{
		Dispositions: []vcl.DispositionSpec{
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Values:    []string{"PRI"},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvSynth,
					Status: uint16(405),
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.http.Host",
						Compare:   vcl.Exists,
						Negate:    true,
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
					vcl.Condition{
						Comparand: "req.esi_level",
						Compare:   vcl.ReqEqual,
						Count:     &uintZero,
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
					vcl.Condition{
						Comparand: "req.proto",
						Compare:   vcl.ReqPrefix,
						Values:    []string{"HTTP/1.1"},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvSynth,
					Status: uint16(400),
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Negate:    true,
						Values: []string{
							"GET",
							"HEAD",
							"PUT",
							"POST",
							"TRACE",
							"OPTIONS",
							"DELETE",
							"PATCH",
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPipe,
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Negate:    true,
						Values: []string{
							"GET",
							"HEAD",
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPass,
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.http.Cookie",
						Compare:   vcl.Exists,
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPass,
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.http.Authorization",
						Compare:   vcl.Exists,
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPass,
				},
			},
		},
	}},
	{spec: []vcr_v1alpha1.RequestDispSpec{
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.Equal,
					Values:    []string{"CONNECT"},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPipe,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.NotEqual,
					Values: []string{
						"GET",
						"HEAD",
						"PUT",
						"POST",
						"TRACE",
						"OPTIONS",
						"DELETE",
						"PATCH",
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvSynth,
				Status: &status405,
			},
		},
	}, exp: vcl.Spec{
		Dispositions: []vcl.DispositionSpec{
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Values:    []string{"CONNECT"},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPipe,
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Negate:    true,
						Values: []string{
							"GET",
							"HEAD",
							"PUT",
							"POST",
							"TRACE",
							"OPTIONS",
							"DELETE",
							"PATCH",
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvSynth,
					Status: uint16(405),
				},
			},
		},
	}},
	{spec: []vcr_v1alpha1.RequestDispSpec{
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.url",
					Compare:   vcr_v1alpha1.Match,
					Values: []string{
						`\.png$`,
						`\.jpe?g$`,
						`\.css$`,
						`\.js$`,
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvHash,
			},
		},
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.url",
					Compare:   vcr_v1alpha1.Prefix,
					Values: []string{
						"/interactive/",
						"/basket/",
						"/personal/",
						"/dynamic/",
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPass,
			},
		},
	}, exp: vcl.Spec{
		Dispositions: []vcl.DispositionSpec{
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.url",
						Compare:   vcl.ReqMatch,
						Values: []string{
							`\.png$`,
							`\.jpe?g$`,
							`\.css$`,
							`\.js$`,
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvHash,
				},
			},
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.url",
						Compare:   vcl.ReqPrefix,
						Values: []string{
							"/interactive/",
							"/basket/",
							"/personal/",
							"/dynamic/",
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPass,
				},
			},
		},
	}},
	{spec: []vcr_v1alpha1.RequestDispSpec{
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.method",
					Compare:   vcr_v1alpha1.Equal,
					Values:    []string{"PURGE"},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvPurge,
			},
		},
	}, exp: vcl.Spec{
		Dispositions: []vcl.DispositionSpec{
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.method",
						Compare:   vcl.ReqEqual,
						Values:    []string{"PURGE"},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvPurge,
				},
			},
		},
	}},
	{spec: []vcr_v1alpha1.RequestDispSpec{
		vcr_v1alpha1.RequestDispSpec{
			Conditions: []vcr_v1alpha1.ReqCondition{
				vcr_v1alpha1.ReqCondition{
					Comparand: "req.url",
					Compare:   vcr_v1alpha1.NotPrefix,
					Values: []string{
						"/foo/",
						"/bar/",
						"/baz/",
						"/quux/",
					},
				},
			},
			Disposition: vcr_v1alpha1.DispositionSpec{
				Action: vcr_v1alpha1.RecvSynth,
				Status: &status403,
			},
		},
	}, exp: vcl.Spec{
		Dispositions: []vcl.DispositionSpec{
			vcl.DispositionSpec{
				Conditions: []vcl.Condition{
					vcl.Condition{
						Comparand: "req.url",
						Compare:   vcl.ReqPrefix,
						Negate:    true,
						Values: []string{
							"/foo/",
							"/bar/",
							"/baz/",
							"/quux/",
						},
						MatchFlags: vcl.MatchFlagsType{
							CaseSensitive: true,
						},
					},
				},
				Disposition: vcl.DispositionType{
					Action: vcl.RecvSynth,
					Status: uint16(403),
				},
			},
		},
	}},
}

func TestConfigReqDisps(t *testing.T) {
	worker := &NamespaceWorker{log: &logrus.Logger{Out: ioutil.Discard}}
	for _, h := range reqDispHarness {
		vclSpec := &vcl.Spec{}
		worker.configReqDisps(vclSpec, h.spec, "VarnishConfig",
			"namespace", "name")
		if !cmp.Equal(vclSpec.Dispositions, h.exp.Dispositions) {
			diff := cmp.Diff(vclSpec.Dispositions,
				h.exp.Dispositions)
			t.Errorf("configReqDisps(%+v) diff(got, expected)=%s",
				h.spec, diff)
		}
	}
}
