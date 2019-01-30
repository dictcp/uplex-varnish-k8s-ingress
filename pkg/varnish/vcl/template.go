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
	"bytes"
	"fmt"
	"hash/fnv"
	"math/big"
	"path"
	"regexp"
	"strings"
	"text/template"
)

var fMap = template.FuncMap{
	"plusOne":      func(i int) int { return i + 1 },
	"vclMangle":    func(s string) string { return mangle(s) },
	"aclMask":      func(bits uint8) string { return aclMask(bits) },
	"hasXFF":       func(acls []ACL) bool { return hasXFF(acls) },
	"dirType":      func(svc Service) string { return dirType(svc) },
	"rewrName":     func(i int) string { return rewrName(i) },
	"needsMatcher": func(rewr Rewrite) bool { return needsMatcher(rewr) },
	"rewrVMOD":     func(rewr Rewrite) string { return rewrVMOD(rewr) },
	"rewrFlags":    func(rewr Rewrite) string { return rewrFlags(rewr) },
	"needsSave":    func(rewr Rewrite) bool { return needsSave(rewr) },
	"needsCompile": func(rewr Rewrite) bool { return needsCompile(rewr) },
	"rewrSub":      func(rewr Rewrite) string { return rewrSub(rewr) },
	"rewrMatch":    func(rewr Rewrite) string { return rewrMatch(rewr) },
	"rewrOp":       func(rewr Rewrite) string { return rewrOp(rewr) },
	"aclCmp": func(comparand string) string {
		return aclCmp(comparand)
	},
	"cmpRelation": func(cmp CompareType) string {
		return cmpRelation(cmp)
	},
	"backendName": func(svc Service, addr string) string {
		return backendName(svc, addr)
	},
	"dirName": func(svc Service) string {
		return directorName(svc)
	},
	"urlMatcher": func(rule Rule) string {
		return urlMatcher(rule)
	},
	"aclName": func(name string) string {
		return mangle(name + "_acl")
	},
	"probeName": func(name string) string {
		return mangle(name + "_probe")
	},
	"credsMatcher": func(realm string) string {
		return mangle(realm + "_auth")
	},
	"rewrMethodAppend": func(rewr Rewrite) bool {
		return rewr.Method == Append
	},
	"rewrMethodPrepend": func(rewr Rewrite) bool {
		return rewr.Method == Prepend
	},
	"rewrMethodDelete": func(rewr Rewrite) bool {
		return rewr.Method == Delete
	},
	"rewrMethodReplace": func(rewr Rewrite) bool {
		return rewr.Method == Replace
	},
	"needsRegex": func(rewr Rewrite) bool {
		return rewr.Compare != RewriteMatch &&
			(rewr.Method == Sub || rewr.Method == Suball)
	},
	"saveRegex": func(rewr Rewrite, rule RewriteRule) string {
		regex := `^\Q` + rule.Value + `\E`
		if rewr.Compare == Prefix {
			return regex
		}
		return regex + "$"
	},
	"needsAll": func(rewr Rewrite) bool {
		return rewr.Compare != RewriteMatch && rewr.Method == Suball
	},
	"needsNeverCapture": func(rewr Rewrite) bool {
		return rewr.Compare == RewriteMatch &&
			rewr.MatchFlags.NeverCapture
	},
	"rewrOperand1": func(rewr Rewrite) string {
		return rewrOperand1(rewr)
	},
	"rewrOperand2": func(rewr Rewrite, i int) string {
		return rewrOperand2(rewr, i)
	},
}

const (
	ingTmplSrc     = "vcl.tmpl"
	shardTmplSrc   = "self-shard.tmpl"
	authTmplSrc    = "auth.tmpl"
	aclTmplSrc     = "acl.tmpl"
	rewriteTmplSrc = "rewrite.tmpl"

	// maxSymLen is a workaround for Varnish issue #2880
	// https://github.com/varnishcache/varnish-cache/issues/2880
	// Will be unnecssary as of the March 2019 release
	maxSymLen = 48
)

var (
	ingressTmpl *template.Template
	shardTmpl   *template.Template
	authTmpl    *template.Template
	aclTmpl     *template.Template
	rewriteTmpl *template.Template
	vclIllegal  = regexp.MustCompile("[^[:word:]-]+")
)

// InitTemplates initializes templates for VCL generation.
func InitTemplates(tmplDir string) error {
	var err error
	ingTmplPath := path.Join(tmplDir, ingTmplSrc)
	shardTmplPath := path.Join(tmplDir, shardTmplSrc)
	authTmplPath := path.Join(tmplDir, authTmplSrc)
	aclTmplPath := path.Join(tmplDir, aclTmplSrc)
	rewriteTmplPath := path.Join(tmplDir, rewriteTmplSrc)

	ingressTmpl, err = template.New(ingTmplSrc).
		Funcs(fMap).ParseFiles(ingTmplPath)
	if err != nil {
		return err
	}
	shardTmpl, err = template.New(shardTmplSrc).
		Funcs(fMap).ParseFiles(shardTmplPath)
	if err != nil {
		return err
	}
	authTmpl, err = template.New(authTmplSrc).
		Funcs(fMap).ParseFiles(authTmplPath)
	if err != nil {
		return err
	}
	aclTmpl, err = template.New(aclTmplSrc).
		Funcs(fMap).ParseFiles(aclTmplPath)
	if err != nil {
		return err
	}
	rewriteTmpl, err = template.New(rewriteTmplSrc).
		Funcs(fMap).ParseFiles(rewriteTmplPath)
	if err != nil {
		return err
	}
	return nil
}

func replIllegal(ill []byte) []byte {
	repl := []byte("_")
	for _, b := range ill {
		repl = append(repl, []byte(fmt.Sprintf("%02x", b))...)
	}
	repl = append(repl, []byte("_")...)
	return repl
}

// GetSrc returns the VCL generated to implement a Spec.
func (spec Spec) GetSrc() (string, error) {
	var buf bytes.Buffer
	if err := ingressTmpl.Execute(&buf, spec); err != nil {
		return "", err
	}
	if len(spec.ShardCluster.Nodes) > 0 {
		if err := shardTmpl.Execute(&buf, spec.ShardCluster); err != nil {
			return "", err
		}
	}
	if len(spec.ACLs) > 0 {
		if err := aclTmpl.Execute(&buf, spec); err != nil {
			return "", err
		}
	}
	if len(spec.Auths) > 0 {
		if err := authTmpl.Execute(&buf, spec); err != nil {
			return "", err
		}
	}
	if len(spec.Rewrites) > 0 {
		if err := rewriteTmpl.Execute(&buf, spec); err != nil {
			return "", err
		}
	}
	if spec.VCL != "" {
		buf.WriteString(spec.VCL)
	}
	return buf.String(), nil
}

func bound(s string, l int) string {
	if len(s) <= l {
		return s
	}
	b := []byte(s)
	h := fnv.New32a()
	h.Write(b)
	i := big.NewInt(int64(h.Sum32()))
	b[l-7] = byte('_')
	copy(b[l-6:l], []byte(i.Text(62)))
	return string(b[0:l])
}

func mangle(s string) string {
	if s == "" {
		return s
	}
	prefixed := "vk8s_" + s
	bytes := []byte(prefixed)
	mangled := vclIllegal.ReplaceAllFunc(bytes, replIllegal)
	return bound(string(mangled), maxSymLen)
}

func backendName(svc Service, addr string) string {
	return mangle(svc.Name + "_" + strings.Replace(addr, ".", "_", -1))
}

func directorName(svc Service) string {
	return mangle(svc.Name + "_director")
}

func urlMatcher(rule Rule) string {
	return mangle(strings.Replace(rule.Host, ".", "_", -1) + "_url")
}

func aclMask(bits uint8) string {
	if bits > 128 {
		return ""
	}
	return fmt.Sprintf("/%d", bits)
}

const (
	xffFirst   = `regsub(req.http.X-Forwarded-For,"^([^,\s]+).*","\1")`
	xff2ndLast = `regsub(req.http.X-Forwarded-For,"^.*?([[:xdigit:]:.]+)\s*,[^,]*$","\1")`
)

func aclCmp(comparand string) string {
	if strings.HasPrefix(comparand, "xff-") ||
		strings.HasPrefix(comparand, "req.http.") {

		if comparand == "xff-first" {
			comparand = xffFirst
		} else if comparand == "xff-2ndlast" {
			comparand = xff2ndLast
		}
		return fmt.Sprintf(`std.ip(%s, "0.0.0.0")`, comparand)
	}
	return comparand
}

func hasXFF(acls []ACL) bool {
	for _, acl := range acls {
		if strings.HasPrefix(acl.Comparand, "xff-") {
			return true
		}
	}
	return false
}

func cmpRelation(cmp CompareType) string {
	switch cmp {
	case Equal:
		return "=="
	case NotEqual:
		return "!="
	case Match:
		return "~"
	case NotMatch:
		return "!~"
	default:
		return "__INVALID_COMPARISON_TYPE__"
	}
}

func dirType(svc Service) string {
	if svc.Director == nil {
		return RoundRobin.String()
	}
	return svc.Director.Type.String()
}

func needsMatcher(rewr Rewrite) bool {
	switch rewr.Method {
	case Append, Prepend, Delete, Replace:
		if len(rewr.Rules) == 0 ||
			(len(rewr.Rules) == 1 && rewr.Rules[0].Value == "") {
			return false
		}
		return true
	default:
		return true
	}
}

func rewrName(i int) string {
	return fmt.Sprintf("vk8s_rewrite_%d", i)
}

func rewrVMOD(rewr Rewrite) string {
	if rewr.Compare == RewriteMatch {
		return "re2"
	}
	return "selector"
}

func rewrFlags(rewr Rewrite) string {
	if rewr.Compare != RewriteMatch {
		if !rewr.MatchFlags.CaseSensitive {
			return "case_sensitive=false"
		}
		return ""
	}

	var flags []string
	if rewr.MatchFlags.MaxMem != 0 && rewr.MatchFlags.MaxMem != 8388608 {
		maxMem := fmt.Sprintf("max_mem=%d", rewr.MatchFlags.MaxMem)
		flags = append(flags, maxMem)
	}
	if rewr.MatchFlags.Anchor != None {
		switch rewr.MatchFlags.Anchor {
		case Start:
			flags = append(flags, "anchor=start")
		case Both:
			flags = append(flags, "anchor=both")
		}
	}
	if rewr.MatchFlags.UTF8 {
		flags = append(flags, "utf8=true")
	}
	if rewr.MatchFlags.PosixSyntax {
		flags = append(flags, "posix_syntax=true")
	}
	if rewr.MatchFlags.LongestMatch {
		flags = append(flags, "longest_match=true")
	}
	if rewr.MatchFlags.Literal {
		flags = append(flags, "literal=true")
	}
	if !rewr.MatchFlags.CaseSensitive {
		flags = append(flags, "case_sensitive=false")
	}
	if rewr.MatchFlags.PerlClasses {
		flags = append(flags, "perl_classes=true")
	}
	if rewr.MatchFlags.WordBoundary {
		flags = append(flags, "word_boundary=true")
	}
	return strings.Join(flags, ",")
}

func needsSave(rewr Rewrite) bool {
	if rewr.Compare != RewriteMatch {
		return false
	}
	switch rewr.Method {
	case Sub, Suball, RewriteMethod:
		return true
	default:
		return false
	}
}

func needsCompile(rewr Rewrite) bool {
	return rewr.Compare == RewriteMatch
}

func rewrSub(rewr Rewrite) string {
	if rewr.VCLSub == Unspecified {
		if strings.HasPrefix(rewr.Target, "resp") ||
			strings.HasPrefix(rewr.Target, "resp") {
			rewr.VCLSub = Deliver
		} else if strings.HasPrefix(rewr.Target, "beresp") ||
			strings.HasPrefix(rewr.Target, "beresp") {
			rewr.VCLSub = BackendResponse
		} else if strings.HasPrefix(rewr.Target, "req") {
			rewr.VCLSub = Recv
		} else {
			rewr.VCLSub = BackendFetch
		}
	}
	switch rewr.VCLSub {
	case Recv:
		return "recv"
	case Pipe:
		return "pipe"
	case Pass:
		return "pass"
	case Hash:
		return "hash"
	case Purge:
		return "purge"
	case Miss:
		return "miss"
	case Hit:
		return "hit"
	case Deliver:
		return "deliver"
	case Synth:
		return "synth"
	case BackendFetch:
		return "backend_fetch"
	case BackendResponse:
		return "backend_response"
	case BackendError:
		return "backend_error"
	default:
		return "__UNKNOWN_VCL_SUB__"
	}
}

func rewrOperand1(rewr Rewrite) string {
	if len(rewr.Rules) == 0 {
		return rewr.Target
	}
	return rewr.Source
}

func rewrOperand2(rewr Rewrite, i int) string {
	if len(rewr.Rules) == 1 && rewr.Rules[0].Value == "" {
		return `"` + rewr.Rules[0].Rewrite + `"`
	}
	if len(rewr.Rules) > 0 && rewr.Rules[0].Value != "" {
		return rewrName(i) + ".string()"
	}
	return rewr.Source
}

func rewrMatch(rewr Rewrite) string {
	switch rewr.Compare {
	case RewriteMatch, RewriteEqual:
		return "match"
	case Prefix:
		return "hasprefix"
	default:
		return "__INVALID_MATCH_OPERATION__"
	}
}

func rewrOp(rewr Rewrite) string {
	switch rewr.Method {
	case Sub:
		return "sub"
	case Suball:
		if rewr.Compare == RewriteMatch {
			return "suball"
		}
		return "sub"
	case RewriteMethod:
		return "extract"
	default:
		return "__INVALID_REWRITE_OPERAION__"
	}
}
