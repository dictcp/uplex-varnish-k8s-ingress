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
	"io/ioutil"
)

func testTemplate(t *testing.T, spec Spec, gold string) {
	var buf bytes.Buffer

	if err := rewriteTmpl.Execute(&buf, spec); err != nil {
		t.Fatal("Execute():", err)
	}

	ok, err := cmpGold(buf.Bytes(), gold)
	if err != nil {
		t.Fatalf("Reading %s: %v", gold, err)
	}
	if !ok {
		t.Errorf("Generated VCL does not match gold file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", buf.String())
		}
	}
}

var replaceFromStringTest = Spec{
	Rewrites: []Rewrite{{
		Rules: []RewriteRule{{
			Rewrite: "baz",
		}},
		Target: "beresp.http.X-Foo",
		Method: Replace,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestReplaceFromString(t *testing.T) {
	gold := "rewrite_replace_from_string.golden"
	testTemplate(t, replaceFromStringTest, gold)
}

var replaceFromSourceTest = Spec{
	Rewrites: []Rewrite{{
		Target: "resp.http.X-Foo",
		Source: "req.http.X-Foo",
		Method: Replace,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestReplaceFromSource(t *testing.T) {
	gold := "rewrite_replace_from_source.golden"
	testTemplate(t, replaceFromSourceTest, gold)
}

var replaceFromRewriteTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "req.http.Host",
		Source:  "req.http.Host",
		Compare: RewriteEqual,
		Rules: []RewriteRule{
			{
				Value:   "cafe.example.com",
				Rewrite: "my-cafe.com",
			},
			{
				Value:   "another.example.com",
				Rewrite: "my-example.com",
			},
		},
		Method: Replace,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestReplaceFromRewrite(t *testing.T) {
	gold := "rewrite_replace_from_rewrite.golden"
	testTemplate(t, replaceFromRewriteTest, gold)
}

var rewriteSubTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "req.url",
		Source:  "req.url",
		Compare: RewriteMatch,
		Rules: []RewriteRule{
			{
				Value:   `/foo(/|$)`,
				Rewrite: `/bar\1`,
			},
			{
				Value:   `/baz(/|$)`,
				Rewrite: `/quux\1`,
			},
		},
		Method: Sub,
		MatchFlags: MatchFlagsType{
			Anchor:        Start,
			CaseSensitive: true,
		},
	}},
}

func TestRewriteSub(t *testing.T) {
	gold := "rewrite_sub.golden"
	testTemplate(t, rewriteSubTest, gold)
}

var rewriteAppendTest = Spec{
	Rewrites: []Rewrite{{
		Source: "req.http.X-Foo",
		Target: "req.http.X-Quux",
		Rules: []RewriteRule{{
			Rewrite: "baz",
		}},
		Method: Append,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteAppend(t *testing.T) {
	gold := "rewrite_append.golden"
	testTemplate(t, rewriteAppendTest, gold)
}

var rewritePrependTest = Spec{
	Rewrites: []Rewrite{{
		Target: "req.http.X-Quux",
		Source: "req.http.X-Quux",
		Rules: []RewriteRule{{
			Rewrite: "baz",
		}},
		Method: Prepend,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewritePrepend(t *testing.T) {
	gold := "rewrite_prepend.golden"
	testTemplate(t, rewritePrependTest, gold)
}

var rewriteDeleteTest = Spec{
	Rewrites: []Rewrite{{
		Target: "req.http.X-Quux",
		Method: Delete,
		VCLSub: Miss,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteDelete(t *testing.T) {
	gold := "rewrite_delete.golden"
	testTemplate(t, rewriteDeleteTest, gold)
}

var conditionalDeleteTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "resp.http.Via",
		Source:  "req.http.Delete-Via",
		Method:  Delete,
		Compare: RewriteEqual,
		Rules: []RewriteRule{
			{Value: "true"},
			{Value: "yes"},
			{Value: "on"},
			{Value: "1"},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: false,
		},
	}},
}

func TestConditionalDelete(t *testing.T) {
	gold := "rewrite_conditional_delete.golden"
	testTemplate(t, conditionalDeleteTest, gold)
}

var rewriteExtractTest = Spec{
	Rewrites: []Rewrite{{
		Target: "req.url",
		Source: "req.url",
		Method: RewriteMethod,
		Rules: []RewriteRule{{
			Value:   `/([^/]+)/([^/]+)(.*)`,
			Rewrite: `/\2/\1\3`,
		}},
		MatchFlags: MatchFlagsType{
			Anchor:        Both,
			CaseSensitive: true,
		},
	}},
}

func TestRewriteExtract(t *testing.T) {
	gold := "rewrite_extract.golden"
	testTemplate(t, rewriteExtractTest, gold)
}

var rewriteFixedPrefixTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "bereq.url",
		Source:  "bereq.url",
		Compare: Prefix,
		Method:  Sub,
		Rules: []RewriteRule{
			{
				Value:   `/foo/`,
				Rewrite: `/bar/`,
			},
			{
				Value:   `/baz/`,
				Rewrite: `/quux/`,
			},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteFixedPrefix(t *testing.T) {
	gold := "rewrite_fixed_prefix.golden"
	testTemplate(t, rewriteFixedPrefixTest, gold)
}

var rewriteFixedEqualTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "bereq.url",
		Source:  "bereq.url",
		Compare: RewriteEqual,
		Method:  Sub,
		Rules: []RewriteRule{
			{
				Value:   `/foo/`,
				Rewrite: `/bar/`,
			},
			{
				Value:   `/baz/`,
				Rewrite: `/quux/`,
			},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteFixedEqual(t *testing.T) {
	gold := "rewrite_fixed_equal.golden"
	testTemplate(t, rewriteFixedEqualTest, gold)
}

var rewriteFixedSuballTest = Spec{
	Rewrites: []Rewrite{{
		Target:  "bereq.url",
		Source:  "bereq.url",
		Compare: Prefix,
		Method:  Suball,
		Rules: []RewriteRule{
			{
				Value:   `/foo/`,
				Rewrite: `/bar/`,
			},
			{
				Value:   `/baz/`,
				Rewrite: `/quux/`,
			},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteFixedSuball(t *testing.T) {
	gold := "rewrite_fixed_suball.golden"
	testTemplate(t, rewriteFixedSuballTest, gold)
}

var rewritePrefixRegex = Spec{
	Rewrites: []Rewrite{{
		Target:  "bereq.url",
		Source:  "bereq.url",
		Compare: RewriteMatch,
		Method:  Sub,
		Rules: []RewriteRule{
			{
				Value:   `/foo/`,
				Rewrite: `/bar/`,
			},
			{
				Value:   `/baz/`,
				Rewrite: `/quux/`,
			},
		},
		MatchFlags: MatchFlagsType{
			Anchor:        Start,
			NeverCapture:  true,
			CaseSensitive: true,
		},
	}},
}

func TestRewritePrefixRegex(t *testing.T) {
	gold := "rewrite_prefix_regex.golden"
	testTemplate(t, rewritePrefixRegex, gold)
}

var rewritePrependIfExists = Spec{
	Rewrites: []Rewrite{{
		Target:  "resp.http.X-Bazz",
		Source:  "req.http.X-Bazz",
		Compare: RewriteMatch,
		Method:  Prepend,
		Rules: []RewriteRule{
			{
				Value:   `.`,
				Rewrite: `bazz`,
			},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewritePrependIfExists(t *testing.T) {
	gold := "rewrite_prepend_if_exists.golden"
	testTemplate(t, rewritePrependIfExists, gold)
}

var rewriteExtractCookie = Spec{
	Rewrites: []Rewrite{{
		Target:  "req.http.Session-Token",
		Source:  "req.http.Cookie",
		Compare: RewriteMatch,
		Method:  RewriteMethod,
		Rules: []RewriteRule{{
			Value:   `\bmysession\s*=\s*([^,;[:space:]]+)`,
			Rewrite: `\1`,
		}},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestExtractCookie(t *testing.T) {
	gold := "rewrite_extract_cookie.golden"
	testTemplate(t, rewriteExtractCookie, gold)
}

var rewriteXCacheHdr = Spec{
	Rewrites: []Rewrite{
		{
			Target: "req.http.X-Cache",
			VCLSub: Hit,
			Method: Replace,
			Rules: []RewriteRule{{
				Rewrite: "HIT",
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "req.http.X-Cache",
			VCLSub: Miss,
			Method: Replace,
			Rules: []RewriteRule{{
				Rewrite: "MISS",
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "req.http.X-Cache",
			VCLSub: Pass,
			Method: Replace,
			Rules: []RewriteRule{{
				Rewrite: "MISS",
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "resp.http.X-Cache",
			Source: "req.http.X-Cache",
			Method: Replace,
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
	},
}

func TestXCacheHdr(t *testing.T) {
	gold := "rewrite_x_cache_hdr.golden"
	testTemplate(t, rewriteXCacheHdr, gold)
}

var rewriteAppendFromSrcTest = Spec{
	Rewrites: []Rewrite{{
		Source: "req.http.Append-Hdr-Src",
		Target: "resp.http.Append-Hdr-Target",
		Method: Append,
	}},
}

func TestRewriteAppendFromSrcTest(t *testing.T) {
	gold := "rewrite_append_from_src.golden"
	testTemplate(t, rewriteAppendFromSrcTest, gold)
}

var rewriteAppendRule = Spec{
	Rewrites: []Rewrite{{
		Target: "resp.http.Append-Rule-Target",
		Source: "req.http.Append-Rule-Src",
		Method: Append,
		Rules: []RewriteRule{{
			Value:   `.`,
			Rewrite: `AppendString`,
		}},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteAppendRule(t *testing.T) {
	gold := "rewrite_append_rule.golden"
	testTemplate(t, rewriteAppendRule, gold)
}

var rewritePrependHdr = Spec{
	Rewrites: []Rewrite{{
		Target: "resp.http.Prepend-Hdr-Target",
		Source: "req.http.Prepend-Hdr-Src",
		Method: Prepend,
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewritePrependHdr(t *testing.T) {
	gold := "rewrite_prepend_hdr.golden"
	testTemplate(t, rewritePrependHdr, gold)
}

var rewriteSelectFirst = Spec{
	Rewrites: []Rewrite{{
		Target:  "bereq.http.Hdr",
		Source:  "bereq.url",
		Compare: Prefix,
		Select:  First,
		Rules: []RewriteRule{
			{
				Value:   `/tea/foo/bar/baz/quux`,
				Rewrite: `Quux`,
			},
			{
				Value:   `/tea/foo/bar/baz`,
				Rewrite: `Baz`,
			},
			{
				Value:   `/tea/foo/bar`,
				Rewrite: `Bar`,
			},
			{
				Value:   `/tea/foo`,
				Rewrite: `Foo`,
			},
		},
		MatchFlags: MatchFlagsType{
			CaseSensitive: true,
		},
	}},
}

func TestRewriteSelectFirst(t *testing.T) {
	gold := "rewrite_select_first.golden"
	testTemplate(t, rewriteSelectFirst, gold)
}

var rewriteSelectPermutations = Spec{
	Rewrites: []Rewrite{
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Unique,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  First,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Last,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Exact,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Shortest,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "req.url",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Longest,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `bar`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
	},
}

func TestRewriteSelectPermutations(t *testing.T) {
	gold := "rewrite_select_permute.golden"
	testTemplate(t, rewriteSelectPermutations, gold)
}

var rewriteSelectOperations = Spec{
	Rewrites: []Rewrite{
		{
			Target:  "resp.http.Hdr",
			Source:  "req.url",
			Compare: Prefix,
			Select:  First,
			Method:  Sub,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `foo`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target:  "resp.http.Hdr",
			Source:  "req.url",
			Compare: Prefix,
			Select:  Exact,
			Method:  Suball,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `foo`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "resp.http.Hdr",
			Source: "req.url",
			Select: First,
			Method: Sub,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `foo`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "resp.http.Hdr",
			Source: "req.url",
			Select: Last,
			Method: Suball,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `foo`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
		{
			Target: "resp.http.Hdr",
			Source: "req.url",
			Select: First,
			Method: RewriteMethod,
			Rules: []RewriteRule{{
				Value:   `/foo`,
				Rewrite: `foo`,
			}},
			MatchFlags: MatchFlagsType{
				CaseSensitive: true,
			},
		},
	},
}

func TestRewriteSelectOperations(t *testing.T) {
	gold := "rewrite_select_ops.golden"
	testTemplate(t, rewriteSelectOperations, gold)
}

// Test the use case that Auth should be executed, but the
// Authorization header must be removed, to prevent return(pass) from
// builtin vcl_recv.  For that, the Authorization header delete must
// run *after* the auth protocol is executed.
var rewriteDeleteAuth = Spec{
	Rewrites: []Rewrite{{
		Target: "req.http.Authorization",
		Method: Delete,
	}},
	Auths: []Auth{{
		Realm:  "foo",
		Status: Basic,
		Credentials: []string{
			"QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			"QWxhZGRpbjpPcGVuU2VzYW1l",
		},
	}},
}

func TestRewriteDeleteAuth(t *testing.T) {
	gold := "rewrite_auth_delete.golden"
	var src string
	var err error
	var goldbytes []byte

	if src, err = rewriteDeleteAuth.GetSrc(); err != nil {
		t.Fatal("GetSrc():", err)
	}

	if goldbytes, err = ioutil.ReadFile("testdata/"+gold); err != nil {
		t.Fatal("WriteFile():", err)
	}
	if !bytes.Equal(goldbytes, []byte(src)) {
		t.Fatalf("Generated VCL does not match gold file: %s", gold)
		if testing.Verbose() {
			t.Logf("Generated: %s", src)
		}
	}
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
