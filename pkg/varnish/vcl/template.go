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
	"path"
	"regexp"
	"strings"
	"text/template"
)

var fMap = template.FuncMap{
	"plusOne":     func(i int) int { return i + 1 },
	"vclMangle":   func(s string) string { return mangle(s) },
	"aclMask":     func(bits uint8) string { return aclMask(bits) },
	"aclCmp":      func(comparand string) string { return aclCmp(comparand) },
	"hasXFF":      func(acls []ACL) bool { return hasXFF(acls) },
	"cmpRelation": func(cmp CompareType) string { return cmpRelation(cmp) },
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
		return mangle(name) + "_acl"
	},
}

const (
	ingTmplSrc   = "vcl.tmpl"
	shardTmplSrc = "self-shard.tmpl"
	authTmplSrc  = "auth.tmpl"
	aclTmplSrc   = "acl.tmpl"
)

var (
	ingressTmpl *template.Template
	shardTmpl   *template.Template
	authTmpl    *template.Template
	aclTmpl     *template.Template
	vclIllegal  = regexp.MustCompile("[^[:word:]-]+")
)

// InitTemplates initializes templates for VCL generation.
func InitTemplates(tmplDir string) error {
	var err error
	ingTmplPath := path.Join(tmplDir, ingTmplSrc)
	shardTmplPath := path.Join(tmplDir, shardTmplSrc)
	authTmplPath := path.Join(tmplDir, authTmplSrc)
	aclTmplPath := path.Join(tmplDir, aclTmplSrc)

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
	return buf.String(), nil
}

func mangle(s string) string {
	if s == "" {
		return s
	}
	prefixed := "vk8s_" + s
	bytes := []byte(prefixed)
	mangled := vclIllegal.ReplaceAllFunc(bytes, replIllegal)
	return string(mangled)
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
