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

import "testing"

var zero uint

var builtinRecvSpec = Spec{
	Dispositions: []DispositionSpec{
		{
			Conditions: []Condition{{
				Comparand: "req.method",
				Compare:   ReqEqual,
				Values:    []string{"PRI"},
			}},
			Disposition: DispositionType{
				Action: RecvSynth,
				Status: 405,
			},
		},
		{
			Conditions: []Condition{
				{
					Comparand: "req.http.Host",
					Compare:   Exists,
					Negate:    true,
				},
				{
					Comparand: "req.esi_level",
					Compare:   ReqEqual,
					Count:     &zero,
				},
				{
					Comparand: "req.proto",
					Compare:   ReqPrefix,
					Values:    []string{"HTTP/1.1"},
					MatchFlags: MatchFlagsType{
						CaseSensitive: false,
					},
				},
			},
			Disposition: DispositionType{
				Action: RecvSynth,
				Status: 400,
			},
		},
		{
			Conditions: []Condition{{
				Comparand: "req.method",
				Compare:   ReqEqual,
				Negate:    true,
				Values: []string{
					"GET",
					"HEAD",
					"PUT",
					"POST",
					"TRACE",
					"OPTIONS",
					"DELETE",
				},
				MatchFlags: MatchFlagsType{
					CaseSensitive: true,
				},
			}},
			Disposition: DispositionType{
				Action: RecvPipe,
			},
		},
		{
			Conditions: []Condition{{
				Comparand: "req.method",
				Compare:   ReqEqual,
				Negate:    true,
				Values: []string{
					"GET",
					"HEAD",
				},
				MatchFlags: MatchFlagsType{
					CaseSensitive: true,
				},
			}},
			Disposition: DispositionType{
				Action: RecvPass,
			},
		},
		{
			Conditions: []Condition{{
				Comparand: "req.http.Cookie",
				Compare:   Exists,
			}},
			Disposition: DispositionType{
				Action: RecvPass,
			},
		},
		{
			Conditions: []Condition{{
				Comparand: "req.http.Authorization",
				Compare:   Exists,
			}},
			Disposition: DispositionType{
				Action: RecvPass,
			},
		},
	},
}

func TestReqDispBuiltinRecv(t *testing.T) {
	gold := "recv_disp_builtin.golden"
	testTemplate(t, reqDispTmpl, builtinRecvSpec, gold)
}

var pipeOnConnectSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.method",
			Compare:   ReqEqual,
			Values:    []string{"CONNECT"},
		}},
		Disposition: DispositionType{
			Action: RecvPipe,
		},
	}},
}

func TestReqDispPipeOnConnect(t *testing.T) {
	gold := "recv_disp_pipe_on_connect.golden"
	testTemplate(t, reqDispTmpl, pipeOnConnectSpec, gold)
}

var methodNotAllowedSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.method",
			Compare:   ReqEqual,
			Negate:    true,
			Values: []string{
				"GET",
				"HEAD",
				"PUT",
				"POST",
				"TRACE",
				"OPTIONS",
				"DELETE",
			},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		}},
		Disposition: DispositionType{
			Action: RecvSynth,
			Status: 405,
		},
	}},
}

func TestReqDispMethodNotAllowed(t *testing.T) {
	gold := "recv_disp_method_not_allowed.golden"
	testTemplate(t, reqDispTmpl, methodNotAllowedSpec, gold)
}

var urlWhitelistSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.url",
			Compare:   ReqPrefix,
			Negate:    true,
			Values: []string{
				"/foo",
				"/bar",
				"/baz",
				"/quux",
			},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		}},
		Disposition: DispositionType{
			Action: RecvSynth,
			Status: 403,
		},
	}},
}

func TestReqDispURLWhitelist(t *testing.T) {
	gold := "recv_disp_url_whitelist.golden"
	testTemplate(t, reqDispTmpl, urlWhitelistSpec, gold)
}

var purgeMethodSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.method",
			Compare:   ReqEqual,
			Values:    []string{"PURGE"},
		}},
		Disposition: DispositionType{
			Action: RecvPurge,
		},
	}},
}

func TestReqDispPurgeMethod(t *testing.T) {
	gold := "recv_disp_purge_method.golden"
	testTemplate(t, reqDispTmpl, purgeMethodSpec, gold)
}

var cacheableSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.url",
			Compare:   ReqMatch,
			Values: []string{
				`\.png$`,
				`\.jpe?g$`,
				`\.css$`,
				`\.js$`,
			},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		}},
		Disposition: DispositionType{
			Action: RecvHash,
		},
	}},
}

func TestReqDispCacheable(t *testing.T) {
	gold := "recv_disp_cacheable.golden"
	testTemplate(t, reqDispTmpl, cacheableSpec, gold)
}

var nonCacheableSpec = Spec{
	Dispositions: []DispositionSpec{{
		Conditions: []Condition{{
			Comparand: "req.url",
			Compare:   ReqPrefix,
			Values: []string{
				"/interactive/",
				"/basket/",
				"/personal",
				"/dynamic/",
			},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		}},
		Disposition: DispositionType{
			Action: RecvPass,
		},
	}},
}

func TestReqDispNonCacheable(t *testing.T) {
	gold := "recv_disp_non_cacheable.golden"
	testTemplate(t, reqDispTmpl, nonCacheableSpec, gold)
}

// Code boilerplate for writing the golden file.
// import ioutils
// func TestRewriteXXX(t *testing.T) {
// gold := "rewrite_XXX.golden"
// var buf bytes.Buffer

// if err := rewriteTmpl.Execute(&buf, spec); err != nil {
// 	t.Fatal("Execute():", err)
// }

// if err := ioutil.WriteFile("testdata/"+gold, buf.Bytes(), 0644); err != nil {
// 		t.Fatal("WriteFile():", err)
// 	}
// if testing.Verbose() {
// 	t.Logf("Generated: %s", buf.String())
// }
// }
