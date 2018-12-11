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
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"regexp"
	"sort"
	"text/template"
)

type Address struct {
	IP   string
	Port int32
}

func (addr Address) hash(hash hash.Hash) {
	portBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(portBytes, uint32(addr.Port))
	hash.Write([]byte(addr.IP))
	hash.Write(portBytes)
}

// interface for sorting []Address
type ByIPPort []Address

func (a ByIPPort) Len() int      { return len(a) }
func (a ByIPPort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByIPPort) Less(i, j int) bool {
	if a[i].IP < a[j].IP {
		return true
	}
	return a[i].Port < a[j].Port
}

type Service struct {
	Name      string
	Addresses []Address
}

func (svc Service) hash(hash hash.Hash) {
	hash.Write([]byte(svc.Name))
	for _, addr := range svc.Addresses {
		addr.hash(hash)
	}
}

// interface for sorting []Service
type ByName []Service

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

type Rule struct {
	Host    string
	PathMap map[string]Service
}

func (rule Rule) hash(hash hash.Hash) {
	hash.Write([]byte(rule.Host))
	paths := make([]string, len(rule.PathMap))
	i := 0
	for k := range rule.PathMap {
		paths[i] = k
		i++
	}
	sort.Strings(paths)
	for _, p := range paths {
		hash.Write([]byte(p))
		rule.PathMap[p].hash(hash)
	}
}

// interface for sorting []Rule
type ByHost []Rule

func (a ByHost) Len() int           { return len(a) }
func (a ByHost) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByHost) Less(i, j int) bool { return a[i].Host < a[j].Host }

type Spec struct {
	DefaultService Service
	Rules          []Rule
	AllServices    map[string]Service
	ClusterNodes   []Service
}

func (spec Spec) DeepHash() uint64 {
	hash := fnv.New64a()
	spec.DefaultService.hash(hash)
	for _, rule := range spec.Rules {
		rule.hash(hash)
	}
	svcs := make([]string, len(spec.AllServices))
	i := 0
	for k := range spec.AllServices {
		svcs[i] = k
		i++
	}
	sort.Strings(svcs)
	for _, svc := range svcs {
		hash.Write([]byte(svc))
		spec.AllServices[svc].hash(hash)
	}
	for _, node := range spec.ClusterNodes {
		node.hash(hash)
	}
	return hash.Sum64()
}

func (spec Spec) Canonical() Spec {
	canon := Spec{
		DefaultService: Service{Name: spec.DefaultService.Name},
		Rules:          make([]Rule, len(spec.Rules)),
		AllServices:    make(map[string]Service, len(spec.AllServices)),
		ClusterNodes:   make([]Service, len(spec.ClusterNodes)),
	}
	copy(canon.DefaultService.Addresses, spec.DefaultService.Addresses)
	sort.Stable(ByIPPort(canon.DefaultService.Addresses))
	copy(canon.Rules, spec.Rules)
	sort.Stable(ByHost(canon.Rules))
	for _, rule := range canon.Rules {
		for _, svc := range rule.PathMap {
			sort.Stable(ByIPPort(svc.Addresses))
		}
	}
	for name, svcs := range spec.AllServices {
		canon.AllServices[name] = svcs
		sort.Stable(ByIPPort(canon.AllServices[name].Addresses))
	}
	copy(canon.ClusterNodes, spec.ClusterNodes)
	sort.Stable(ByName(canon.ClusterNodes))
	for _, node := range canon.ClusterNodes {
		sort.Stable(ByIPPort(node.Addresses))
	}
	return canon
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

const (
	ingTmplSrc   = "vcl.tmpl"
	shardTmplSrc = "self-shard.tmpl"
)

var (
	IngressTmpl = template.Must(template.New(ingTmplSrc).Funcs(fMap).ParseFiles(ingTmplSrc))
	ShardTmpl   = template.Must(template.New(shardTmplSrc).Funcs(fMap).ParseFiles(shardTmplSrc))
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

func (spec Spec) GetSrc() (string, error) {
	var buf bytes.Buffer
	if err := IngressTmpl.Execute(&buf, spec); err != nil {
		return "", err
	}
	if len(spec.ClusterNodes) > 0 {
		if err := ShardTmpl.Execute(&buf, spec); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
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
