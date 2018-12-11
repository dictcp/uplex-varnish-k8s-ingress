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

package varnish

import (
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/varnish/vcl"
	"code.uplex.de/uplex-varnish/varnishapi/pkg/admin"

	"github.com/sirupsen/logrus"
)

const (
	regularLabel   = "vk8s_regular"
	readinessLabel = "vk8s_readiness"
	readyCfg       = "vk8s_ready"
	notAvailCfg    = "vk8s_notavailable"
	ingressPrefix  = "vk8s_ing_"
)

// XXX make admTimeout configurable
var (
	nonAlNum   = regexp.MustCompile("[^[:alnum:]]+")
	admTimeout = time.Second * 10
)

type VarnishAdmError struct {
	addr string
	err  error
}

func (vadmErr VarnishAdmError) Error() string {
	return fmt.Sprintf("%s: %v", vadmErr.addr, vadmErr.err)
}

type VarnishAdmErrors []VarnishAdmError

func (vadmErrs VarnishAdmErrors) Error() string {
	var sb strings.Builder
	sb.WriteRune('[')
	for _, err := range vadmErrs {
		sb.WriteRune('{')
		sb.WriteString(err.Error())
		sb.WriteRune('}')
	}
	sb.WriteRune(']')
	return sb.String()
}

type vclSpec struct {
	spec vcl.Spec
	key  string
	uid  string
}

func (spec vclSpec) configName() string {
	name := fmt.Sprintf("%s%s_%s_%0x", ingressPrefix, spec.key, spec.uid,
		spec.spec.Canonical().DeepHash())
	return nonAlNum.ReplaceAllLiteralString(name, "_")
}

type varnishSvc struct {
	addr   string
	Banner string
	admMtx *sync.Mutex
}

type VarnishController struct {
	log         *logrus.Logger
	errChan     chan error
	admSecret   []byte
	varnishSvcs map[string][]*varnishSvc
	spec        *vclSpec
}

func NewVarnishController(log *logrus.Logger) *VarnishController {
	vc := VarnishController{}
	vc.varnishSvcs = make(map[string][]*varnishSvc)
	vc.log = log
	return &vc
}

func (vc *VarnishController) Start(errChan chan error) {
	vc.errChan = errChan
	vc.log.Info("Starting Varnish controller")
	fmt.Printf("Varnish controller logging at level: %s\n", vc.log.Level)
	go vc.monitor()
}

func (vc *VarnishController) updateVarnishInstance(svc *varnishSvc,
	cfgName string, vclSrc string) error {

	svc.admMtx.Lock()
	defer svc.admMtx.Unlock()

	vc.log.Debugf("Connect to %s, timeout=%v", svc.addr, admTimeout)
	adm, err := admin.Dial(svc.addr, vc.admSecret, admTimeout)
	if err != nil {
		return VarnishAdmError{addr: svc.addr, err: err}
	}
	defer adm.Close()
	svc.Banner = adm.Banner
	vc.log.Infof("Connected to Varnish admin endpoint at %s", svc.addr)

	loaded, labelled, ready := false, false, false
	vc.log.Debugf("List VCLs at %s", svc.addr)
	vcls, err := adm.VCLList()
	if err != nil {
		return VarnishAdmError{addr: svc.addr, err: err}
	}
	vc.log.Debugf("VCL List at %s: %+v", svc.addr, vcls)
	for _, vcl := range vcls {
		if vcl.Name == cfgName {
			loaded = true
		}
		if vcl.LabelVCL == cfgName &&
			vcl.Name == regularLabel {
			labelled = true
		}
		if vcl.LabelVCL == readyCfg &&
			vcl.Name == readinessLabel {
			ready = true
		}
	}

	if loaded {
		vc.log.Infof("Config %s already loaded at instance %s", cfgName,
			svc.addr)
	} else {
		vc.log.Debugf("Load config %s at %s", cfgName, svc.addr)
		err = adm.VCLInline(cfgName, vclSrc)
		if err != nil {
			vc.log.Debugf("Error loading config %s at %s: %v",
				cfgName, svc.addr, err)
			return VarnishAdmError{addr: svc.addr, err: err}
		}
		vc.log.Infof("Loaded config %s at Varnish endpoint %s", cfgName,
			svc.addr)
	}

	if labelled {
		vc.log.Infof("Config %s already labelled as regular at %s",
			cfgName, svc.addr)
	} else {
		vc.log.Debugf("Label config %s as %s at %s", cfgName,
			regularLabel, svc.addr)
		err = adm.VCLLabel(regularLabel, cfgName)
		if err != nil {
			return VarnishAdmError{addr: svc.addr, err: err}
		}
		vc.log.Infof("Labeled config %s as %s at Varnish endpoint %s",
			cfgName, regularLabel, svc.addr)
	}

	if ready {
		vc.log.Infof("Config %s already labelled as ready at %s",
			readyCfg, svc.addr)
	} else {
		vc.log.Debugf("Label config %s as %s at %s", readyCfg,
			readinessLabel, svc.addr)
		err = adm.VCLLabel(readinessLabel, readyCfg)
		if err != nil {
			return VarnishAdmError{addr: svc.addr, err: err}
		}
		vc.log.Infof("Labeled config %s as %s at Varnish endpoint %s",
			readyCfg, readinessLabel, svc.addr)
	}
	return nil
}

func (vc *VarnishController) updateVarnishInstances(svcs []*varnishSvc) error {

	if vc.spec == nil {
		vc.log.Info("Update Varnish instances: Currently no Ingress " +
			"defined")
		return nil
	}

	vclSrc, err := vc.spec.spec.GetSrc()
	if err != nil {
		return err
	}
	cfgName := vc.spec.configName()

	vc.log.Infof("Update Varnish instances: load config %s", cfgName)
	var errs VarnishAdmErrors
	for _, svc := range svcs {
		if e := vc.updateVarnishInstance(svc, cfgName, vclSrc); e != nil {
			admErr := VarnishAdmError{addr: svc.addr, err: err}
			errs = append(errs, admErr)
			continue
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (vc *VarnishController) addVarnishSvc(key string,
	addrs []vcl.Address) error {

	vc.varnishSvcs[key] = make([]*varnishSvc, len(addrs))
	for i, addr := range addrs {
		admAddr := addr.IP + ":" + strconv.Itoa(int(addr.Port))
		svc := varnishSvc{
			addr:   admAddr,
			admMtx: &sync.Mutex{},
		}
		vc.varnishSvcs[key][i] = &svc
	}
	return vc.updateVarnishInstances(vc.varnishSvcs[key])
}

// Label cfg as lbl at Varnish instance svc. If mayClose is true, then
// losing the admin connection is not an error (Varnish may be
// shutting down).
func (vc *VarnishController) setCfgLabel(svc *varnishSvc,
	cfg, lbl string, mayClose bool) error {

	svc.admMtx.Lock()
	defer svc.admMtx.Unlock()

	vc.log.Debugf("Connect to %s, timeout=%v", svc.addr, admTimeout)
	adm, err := admin.Dial(svc.addr, vc.admSecret, admTimeout)
	if err != nil {
		if mayClose {
			vc.log.Warnf("Could not connect to %s: %v", svc.addr,
				err)
			return nil
		}
		return VarnishAdmError{addr: svc.addr, err: err}
	}
	defer adm.Close()
	svc.Banner = adm.Banner
	vc.log.Infof("Connected to Varnish admin endpoint at %s", svc.addr)

	vc.log.Debugf("Set config %s to label %s at %s", svc.addr, cfg, lbl)
	if err := adm.VCLLabel(lbl, cfg); err != nil {
		if err == io.EOF {
			if mayClose {
				vc.log.Warnf("Connection at EOF at %s",
					svc.addr)
				return nil
			}
			return VarnishAdmError{addr: svc.addr, err: err}
		}
	}
	return nil
}

// On Delete for a Varnish instance, we set it to the unready state.
func (vc *VarnishController) removeVarnishInstances(svcs []*varnishSvc) error {
	var errs VarnishAdmErrors

	for _, svc := range svcs {
		if err := vc.setCfgLabel(svc, notAvailCfg, readinessLabel,
			true); err != nil {

			admErr := VarnishAdmError{addr: svc.addr, err: err}
			errs = append(errs, admErr)
			continue
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (vc *VarnishController) updateVarnishSvc(key string,
	addrs []vcl.Address) error {

	var errs VarnishAdmErrors
	var newSvcs, remSvcs, keepSvcs []*varnishSvc
	updateAddrs := make(map[string]struct{})
	prevAddrs := make(map[string]*varnishSvc)

	for _, addr := range addrs {
		key := addr.IP + ":" + strconv.Itoa(int(addr.Port))
		updateAddrs[key] = struct{}{}
	}
	for _, svc := range vc.varnishSvcs[key] {
		prevAddrs[svc.addr] = svc
	}
	for addr := range updateAddrs {
		svc, exists := prevAddrs[addr]
		if exists {
			keepSvcs = append(keepSvcs, svc)
			continue
		}
		newSvc := &varnishSvc{addr: addr}
		newSvcs = append(newSvcs, newSvc)
	}
	for addr, svc := range prevAddrs {
		_, exists := updateAddrs[addr]
		if !exists {
			remSvcs = append(remSvcs, svc)
		}
	}
	vc.varnishSvcs[key] = append(keepSvcs, newSvcs...)

	for _, svc := range remSvcs {
		if err := vc.setCfgLabel(svc, notAvailCfg, readinessLabel,
			true); err != nil {

			admErr := VarnishAdmError{addr: svc.addr, err: err}
			errs = append(errs, admErr)
			continue
		}
	}

	updateErrs := vc.updateVarnishInstances(vc.varnishSvcs[key])
	if updateErrs != nil {
		vadmErrs, ok := updateErrs.(VarnishAdmErrors)
		if ok {
			errs = append(errs, vadmErrs...)
		} else {
			return updateErrs
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (vc *VarnishController) AddOrUpdateVarnishSvc(key string,
	addrs []vcl.Address) error {

	if vc.admSecret == nil {
		return fmt.Errorf("Cannot add or update Varnish service %s: "+
			"no admin secret has been set", key)
	}

	_, ok := vc.varnishSvcs[key]
	if !ok {
		return vc.addVarnishSvc(key, addrs)
	}
	return vc.updateVarnishSvc(key, addrs)
}

func (vc *VarnishController) DeleteVarnishSvc(key string) error {
	svcs, ok := vc.varnishSvcs[key]
	if !ok {
		return nil
	}
	delete(vc.varnishSvcs, key)
	return vc.removeVarnishInstances(svcs)
}

func (vc *VarnishController) Update(key, uid string, spec vcl.Spec) error {

	if vc.spec != nil && vc.spec.key != "" && vc.spec.key != key {
		return fmt.Errorf("Multiple Ingress definitions currently not "+
			"supported: current=%s new=%s", vc.spec.key, key)
	}
	if vc.spec == nil {
		vc.spec = &vclSpec{}
	}
	vc.spec.key = key
	vc.spec.uid = uid
	vc.spec.spec = spec

	var errs VarnishAdmErrors
	for _, svcs := range vc.varnishSvcs {
		updateErrs := vc.updateVarnishInstances(svcs)
		vadmErrs, ok := updateErrs.(VarnishAdmErrors)
		if ok {
			errs = append(errs, vadmErrs...)
			continue
		}
		return updateErrs
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// We currently only support one Ingress definition at a time, so
// deleting the Ingress means that we set Varnish instances to the
// not ready state.
func (vc *VarnishController) DeleteIngress(key string) error {
	if vc.spec != nil && vc.spec.key != "" && vc.spec.key != key {
		return fmt.Errorf("Unknown Ingress %s", key)
	}
	vc.spec = nil

	var errs VarnishAdmErrors
	for _, svcs := range vc.varnishSvcs {
		for _, svc := range svcs {
			if err := vc.setCfgLabel(svc, notAvailCfg,
				readinessLabel, false); err != nil {

				admErr := VarnishAdmError{
					addr: svc.addr,
					err:  err,
				}
				errs = append(errs, admErr)
				continue
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// Currently only one Ingress at a time
func (vc *VarnishController) HasIngress(key, uid string, spec vcl.Spec) bool {
	if vc.spec == nil {
		return false
	}
	return vc.spec.key == key &&
		vc.spec.uid == uid &&
		reflect.DeepEqual(vc.spec.spec.Canonical(), spec.Canonical())
}

func (vc *VarnishController) SetAdmSecret(secret []byte) {
	vc.admSecret = make([]byte, len(secret))
	copy(vc.admSecret, secret)
}

// XXX Controller becomes not ready
func (vc *VarnishController) DeleteAdmSecret() {
	vc.admSecret = nil
}

func (vc *VarnishController) Quit() {
	vc.errChan <- nil
}
