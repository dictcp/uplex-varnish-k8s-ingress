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
	"fmt"
	"regexp"
	"text/template"
)

type Address struct {
	IP   string
	Port int32
}

type Service struct {
	Name      string
	Addresses []Address
}

type Rule struct {
	Host    string
	PathMap map[string]Service
}

type Spec struct {
	DefaultService Service
	Rules          []Rule
	AllServices    map[string]Service
}

var fMap = template.FuncMap{
	"plusOne":   func(i int) int { return i + 1 },
	"vclMangle": func(s string) string { return Mangle(s) },
	"backendName": func(svc Service, addr string) string {
		return BackendName(svc, addr)
	},
	"dirName": func(svc Service) string {
		return DirectorName(svc)
	},
	"urlMatcher": func(rule Rule) string {
		return URLMatcher(rule)
	},
}

const tmplSrc = "vcl.tmpl"

var (
	Tmpl        = template.Must(template.New(tmplSrc).Funcs(fMap).ParseFiles(tmplSrc))
	symPattern  = regexp.MustCompile("^[[:alpha:]][[:word:]-]*$")
	first       = regexp.MustCompile("[[:alpha:]]")
	restIllegal = regexp.MustCompile("[^[:word:]-]+")
)

func replIllegal(ill []byte) []byte {
	repl := []byte("_")
	for _, b := range ill {
		repl = append(repl, []byte(fmt.Sprintf("%02x", b))...)
	}
	repl = append(repl, []byte("_")...)
	return repl
}

func Mangle(s string) string {
	var mangled string
	bytes := []byte(s)
	if s == "" || symPattern.Match(bytes) {
		return s
	}
	mangled = string(bytes[0])
	if !first.Match(bytes[0:1]) {
		mangled = "V" + mangled
	}
	rest := restIllegal.ReplaceAllFunc(bytes[1:], replIllegal)
	mangled = mangled + string(rest)
	return mangled
}

func BackendName(svc Service, addr string) string {
	return Mangle(svc.Name + "_" + addr)
}

func DirectorName(svc Service) string {
	return Mangle(svc.Name + "_director")
}

func URLMatcher(rule Rule) string {
	return Mangle(rule.Host + "_url")
}
