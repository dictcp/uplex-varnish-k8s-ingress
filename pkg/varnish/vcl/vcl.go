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
	"path"
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

type Probe struct {
	Timeout   string
	Interval  string
	Initial   string
	Window    string
	Threshold string
}

func (probe Probe) hash(hash hash.Hash) {
	hash.Write([]byte(probe.Timeout))
	hash.Write([]byte(probe.Interval))
	hash.Write([]byte(probe.Initial))
	hash.Write([]byte(probe.Window))
	hash.Write([]byte(probe.Threshold))
}

type ShardCluster struct {
	Nodes           []Service
	Probe           Probe
	MaxSecondaryTTL string
}

func (shard ShardCluster) hash(hash hash.Hash) {
	for _, node := range shard.Nodes {
		node.hash(hash)
	}
	shard.Probe.hash(hash)
	hash.Write([]byte(shard.MaxSecondaryTTL))
}

type Spec struct {
	DefaultService Service
	Rules          []Rule
	AllServices    map[string]Service
	ShardCluster   ShardCluster
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
	spec.ShardCluster.hash(hash)
	return hash.Sum64()
}

func (spec Spec) Canonical() Spec {
	canon := Spec{
		DefaultService: Service{Name: spec.DefaultService.Name},
		Rules:          make([]Rule, len(spec.Rules)),
		AllServices:    make(map[string]Service, len(spec.AllServices)),
		ShardCluster:   spec.ShardCluster,
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
	sort.Stable(ByName(canon.ShardCluster.Nodes))
	for _, node := range canon.ShardCluster.Nodes {
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
	IngressTmpl *template.Template
	ShardTmpl   *template.Template
	symPattern  = regexp.MustCompile("^[[:alpha:]][[:word:]-]*$")
	first       = regexp.MustCompile("[[:alpha:]]")
	restIllegal = regexp.MustCompile("[^[:word:]-]+")
)

func InitTemplates(tmplDir string) error {
	var err error
	ingTmplPath := path.Join(tmplDir, ingTmplSrc)
	shardTmplPath := path.Join(tmplDir, shardTmplSrc)
	IngressTmpl, err = template.New(ingTmplSrc).
		Funcs(fMap).ParseFiles(ingTmplPath)
	if err != nil {
		return err
	}
	ShardTmpl, err = template.New(shardTmplSrc).
		Funcs(fMap).ParseFiles(shardTmplPath)
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

func (spec Spec) GetSrc() (string, error) {
	var buf bytes.Buffer
	if err := IngressTmpl.Execute(&buf, spec); err != nil {
		return "", err
	}
	if len(spec.ShardCluster.Nodes) > 0 {
		if err := ShardTmpl.Execute(&buf, spec.ShardCluster); err != nil {
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