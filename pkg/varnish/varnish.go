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

// Package varnish encapsulates interaction with Varnish instances to
// transform desired states from Ingress and VarnishConfig configs to
// the actual state of the cluster. Only this package imports
// varnishapi/pkg/admin to interact with the CLI of each Varnish
// instance.
package varnish

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/interfaces"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"
	"code.uplex.de/uplex-varnish/varnishapi/pkg/admin"

	"github.com/prometheus/client_golang/prometheus"
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

// VarnishAdmError encapsulates an error encountered for an individual
// Varnish instance, and satisfies the Error interface.
type VarnishAdmError struct {
	addr string
	err  error
}

// Error returns an error meesage for an error encountered at a
// Varnish instance, identifying the instance by its Endpoint address
// (internal IP) and admin port.
func (vadmErr VarnishAdmError) Error() string {
	return fmt.Sprintf("%s: %v", vadmErr.addr, vadmErr.err)
}

// VarnishAdmErrors is a collection of errors encountered at Varnish
// instances. Most attempts to sync the state of Varnish instances do
// not break off at the first error; the attempt is repeated for each
// instance in a cluster, collecting error information along the way.
// This object contains error information for each instance in a
// cluster that failed to sync. The type satisifies the Error
// interface.
type VarnishAdmErrors []VarnishAdmError

// Error returns an error message that includes errors for each
// instance in a Varnish cluster that failed a sync operation, where
// each instance is identified by it Endpoint (internal IP) and admin
// port.
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

type varnishInst struct {
	addr      string
	admSecret *[]byte
	Banner    string
	admMtx    *sync.Mutex
}

type varnishSvc struct {
	instances []*varnishInst
	spec      *vclSpec
	secrName  string
	cfgLoaded bool
}

// VarnishController encapsulates information about each Varnish
// cluster deployed as Ingress implementations in the cluster, and
// their current states.
type VarnishController struct {
	log      *logrus.Logger
	svcEvt   interfaces.SvcEventGenerator
	svcs     map[string]*varnishSvc
	secrets  map[string]*[]byte
	errChan  chan error
	monIntvl time.Duration
}

// NewVarnishController returns an instance of VarnishController.
//
//    log: logger object initialized at startup
//    tmplDir: directory containing templates for VCL generation
//
// If tmplDir is the empty string, use the environment variable
// TEMPLATE_DIR. If the env variable does not exist, use the current
// working directory.
func NewVarnishController(
	log *logrus.Logger,
	tmplDir string,
	monIntvl time.Duration) (*VarnishController, error) {

	if tmplDir == "" {
		tmplEnv, exists := os.LookupEnv("TEMPLATE_DIR")
		if exists {
			tmplDir = tmplEnv
		}
	}
	if err := vcl.InitTemplates(tmplDir); err != nil {
		return nil, err
	}
	initMetrics()
	return &VarnishController{
		svcs:     make(map[string]*varnishSvc),
		secrets:  make(map[string]*[]byte),
		log:      log,
		monIntvl: monIntvl,
	}, nil
}

// EvtGenerator sets the object that implements interface
// SvcEventGenerator, and will be used by the monitor goroutine to
// generate Events for Varnish Services.
func (vc *VarnishController) EvtGenerator(svcEvt interfaces.SvcEventGenerator) {
	vc.svcEvt = svcEvt
}

// Start initiates the Varnish controller and starts the monitor
// goroutine.
func (vc *VarnishController) Start(errChan chan error) {
	vc.errChan = errChan
	vc.log.Info("Starting Varnish controller")
	fmt.Printf("Varnish controller logging at level: %s\n", vc.log.Level)
	go vc.monitor(vc.monIntvl)
}

func (vc *VarnishController) updateVarnishInstance(inst *varnishInst,
	cfgName string, vclSrc string, metrics *instanceMetrics) error {

	vc.log.Infof("Update Varnish instance at %s", inst.addr)
	vc.log.Debugf("Varnish instance %s: %+v", inst.addr, *inst)
	if inst.admSecret == nil {
		return fmt.Errorf("No known admin secret")
	}
	inst.admMtx.Lock()
	defer inst.admMtx.Unlock()

	vc.log.Debugf("Connect to %s, timeout=%v", inst.addr, admTimeout)
	timer := prometheus.NewTimer(metrics.connectLatency)
	adm, err := admin.Dial(inst.addr, *inst.admSecret, admTimeout)
	timer.ObserveDuration()
	if err != nil {
		metrics.connectFails.Inc()
		return err
	}
	defer adm.Close()
	inst.Banner = adm.Banner
	vc.log.Infof("Connected to Varnish admin endpoint at %s", inst.addr)

	loaded, labelled, ready := false, false, false
	vc.log.Debugf("List VCLs at %s", inst.addr)
	vcls, err := adm.VCLList()
	if err != nil {
		return err
	}
	vc.log.Debugf("VCL List at %s: %+v", inst.addr, vcls)
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
			inst.addr)
	} else {
		vc.log.Debugf("Load config %s at %s", cfgName, inst.addr)
		timer = prometheus.NewTimer(metrics.vclLoadLatency)
		err = adm.VCLInline(cfgName, vclSrc)
		timer.ObserveDuration()
		if err != nil {
			vc.log.Debugf("Error loading config %s at %s: %v",
				cfgName, inst.addr, err)
			metrics.vclLoadErrs.Inc()
			return err
		}
		metrics.vclLoads.Inc()
		vc.log.Infof("Loaded config %s at Varnish endpoint %s", cfgName,
			inst.addr)
	}

	if labelled {
		vc.log.Infof("Config %s already labelled as regular at %s",
			cfgName, inst.addr)
	} else {
		vc.log.Debugf("Label config %s as %s at %s", cfgName,
			regularLabel, inst.addr)
		err = adm.VCLLabel(regularLabel, cfgName)
		if err != nil {
			return err
		}
		vc.log.Infof("Labeled config %s as %s at Varnish endpoint %s",
			cfgName, regularLabel, inst.addr)
	}

	if ready {
		vc.log.Infof("Config %s already labelled as ready at %s",
			readyCfg, inst.addr)
	} else {
		vc.log.Debugf("Label config %s as %s at %s", readyCfg,
			readinessLabel, inst.addr)
		err = adm.VCLLabel(readinessLabel, readyCfg)
		if err != nil {
			return err
		}
		vc.log.Infof("Labeled config %s as %s at Varnish endpoint %s",
			readyCfg, readinessLabel, inst.addr)
	}
	return nil
}

func (vc *VarnishController) updateVarnishSvc(name string) error {
	svc, exists := vc.svcs[name]
	if !exists || svc == nil {
		return fmt.Errorf("No known Varnish Service %s", name)
	}
	vc.log.Debugf("Update Varnish svc %s: config=%+v", name, *svc)
	svc.cfgLoaded = false
	if svc.secrName == "" {
		return fmt.Errorf("No known admin secret for Varnish Service "+
			"%s", name)
	}
	if svc.spec == nil {
		vc.log.Infof("Update Varnish Service %s: Currently no Ingress"+
			" defined", name)
		return nil
	}

	vclSrc, err := svc.spec.spec.GetSrc()
	if err != nil {
		return err
	}
	cfgName := svc.spec.configName()

	vc.log.Infof("Update Varnish instances: load config %s", cfgName)
	var errs VarnishAdmErrors
	for _, inst := range svc.instances {
		if inst == nil {
			vc.log.Errorf("Instance object is nil")
			continue
		}
		metrics := getInstanceMetrics(inst.addr)
		metrics.updates.Inc()
		if e := vc.updateVarnishInstance(inst, cfgName, vclSrc,
			metrics); e != nil {

			admErr := VarnishAdmError{addr: inst.addr, err: e}
			errs = append(errs, admErr)
			metrics.updateErrs.Inc()
			continue
		}
	}
	if len(errs) == 0 {
		svc.cfgLoaded = true
		return nil
	}
	return errs
}

// Label cfg as lbl at Varnish instance inst. If mayClose is true, then
// losing the admin connection is not an error (Varnish may be
// shutting down).
func (vc *VarnishController) setCfgLabel(inst *varnishInst,
	cfg, lbl string, mayClose bool) error {

	if inst.admSecret == nil {
		return VarnishAdmError{
			addr: inst.addr,
			err:  fmt.Errorf("No known admin secret"),
		}
	}
	metrics := getInstanceMetrics(inst.addr)
	inst.admMtx.Lock()
	defer inst.admMtx.Unlock()

	vc.log.Debugf("Connect to %s, timeout=%v", inst.addr, admTimeout)
	timer := prometheus.NewTimer(metrics.connectLatency)
	adm, err := admin.Dial(inst.addr, *inst.admSecret, admTimeout)
	timer.ObserveDuration()
	if err != nil {
		if mayClose {
			vc.log.Warnf("Could not connect to %s: %v", inst.addr,
				err)
			return nil
		}
		metrics.connectFails.Inc()
		return VarnishAdmError{addr: inst.addr, err: err}
	}
	defer adm.Close()
	inst.Banner = adm.Banner
	vc.log.Infof("Connected to Varnish admin endpoint at %s", inst.addr)

	vc.log.Debugf("Set config %s to label %s at %s", inst.addr, cfg, lbl)
	if err := adm.VCLLabel(lbl, cfg); err != nil {
		if err == io.EOF {
			if mayClose {
				vc.log.Warnf("Connection at EOF at %s",
					inst.addr)
				return nil
			}
			return VarnishAdmError{addr: inst.addr, err: err}
		}
	}
	return nil
}

// On Delete for a Varnish instance, we set it to the unready state.
func (vc *VarnishController) removeVarnishInstances(insts []*varnishInst) error {
	var errs VarnishAdmErrors

	for _, inst := range insts {
		// XXX health check for sharding config should fail
		if err := vc.setCfgLabel(inst, notAvailCfg, readinessLabel,
			true); err != nil {

			admErr := VarnishAdmError{addr: inst.addr, err: err}
			errs = append(errs, admErr)
			continue
		}
		instsGauge.Dec()
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (vc *VarnishController) updateVarnishSvcAddrs(key string,
	addrs []vcl.Address, secrPtr *[]byte, loadVCL bool) error {

	var errs VarnishAdmErrors
	var newInsts, remInsts, keepInsts []*varnishInst

	svc, exists := vc.svcs[key]
	if !exists {
		// Shouldn't be possible
		return fmt.Errorf("No known Varnish Service %s", key)
	}

	updateAddrs := make(map[string]struct{})
	prevAddrs := make(map[string]*varnishInst)
	for _, addr := range addrs {
		key := addr.IP + ":" + strconv.Itoa(int(addr.Port))
		updateAddrs[key] = struct{}{}
	}
	for _, inst := range svc.instances {
		prevAddrs[inst.addr] = inst
	}
	for addr := range updateAddrs {
		inst, exists := prevAddrs[addr]
		if exists {
			keepInsts = append(keepInsts, inst)
			continue
		}
		newInst := &varnishInst{
			addr:      addr,
			admSecret: secrPtr,
			admMtx:    &sync.Mutex{},
		}
		newInsts = append(newInsts, newInst)
		instsGauge.Inc()
	}
	for addr, inst := range prevAddrs {
		_, exists := updateAddrs[addr]
		if !exists {
			remInsts = append(remInsts, inst)
		}
	}
	vc.log.Debugf("Varnish svc %s: keeping instances=%+v, "+
		"new instances=%+v, removing instances=%+v", key, keepInsts,
		newInsts, remInsts)
	svc.instances = append(keepInsts, newInsts...)

	for _, inst := range remInsts {
		vc.log.Debugf("Varnish svc %s setting to not ready: %+v", key,
			inst)
		if err := vc.setCfgLabel(inst, notAvailCfg, readinessLabel,
			true); err != nil {

			admErr := VarnishAdmError{addr: inst.addr, err: err}
			errs = append(errs, admErr)
			continue
		}
		instsGauge.Dec()
	}
	vc.log.Debugf("Varnish svc %s config: %+v", key, *svc)

	if loadVCL {
		vc.log.Debugf("Varnish svc %s: load VCL", key)
		updateErrs := vc.updateVarnishSvc(key)
		if updateErrs != nil {
			vadmErrs, ok := updateErrs.(VarnishAdmErrors)
			if ok {
				errs = append(errs, vadmErrs...)
			} else {
				return updateErrs
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// AddOrUpdateVarnishSvc causes a sync for the Varnish Service
// identified by namespace/name key.
//
//    addrs: list of admin addresses for instances in the Service
//           (internal IPs and admin ports)
//    secrName: namespace/name of the admin secret to use for the
//              Service
//    loadVCL: true if the VCL config for the Service should be
//             reloaded
func (vc *VarnishController) AddOrUpdateVarnishSvc(key string,
	addrs []vcl.Address, secrName string, loadVCL bool) error {

	var secrPtr *[]byte
	svc, svcExists := vc.svcs[key]
	if !svcExists {
		var instances []*varnishInst
		svc = &varnishSvc{}
		for _, addr := range addrs {
			admAddr := addr.IP + ":" + strconv.Itoa(int(addr.Port))
			instance := &varnishInst{
				addr:   admAddr,
				admMtx: &sync.Mutex{},
			}
			vc.log.Debugf("Varnish svc %s: creating instance %+v",
				key, *instance)
			instances = append(instances, instance)
			instsGauge.Inc()
		}
		svc.instances = instances
		vc.svcs[key] = svc
		svcsGauge.Inc()
		vc.log.Debugf("Varnish svc %s: created config", key)
	}
	vc.log.Debugf("Varnish svc %s config: %+v", key, svc)

	svc.secrName = secrName
	if _, exists := vc.secrets[secrName]; exists {
		secrPtr = vc.secrets[secrName]
	} else {
		secrPtr = nil
	}
	for _, inst := range svc.instances {
		inst.admSecret = secrPtr
	}
	vc.log.Debugf("Varnish svc %s: updated instance with secret %s", key,
		secrName)

	vc.log.Debugf("Update Varnish svc %s: addrs=%+v secret=%s reloadVCL=%v",
		key, addrs, secrName, loadVCL)
	return vc.updateVarnishSvcAddrs(key, addrs, secrPtr, loadVCL)
}

// DeleteVarnishSvc is called on the Delete event for the Varnish
// Service identified by the namespace/name key. The Varnish instance
// is set to the unready state, and no further action is taken (other
// resources in the cluster may shut down the Varnish instances).
func (vc *VarnishController) DeleteVarnishSvc(key string) error {
	svc, ok := vc.svcs[key]
	if !ok {
		return nil
	}
	err := vc.removeVarnishInstances(svc.instances)
	if err != nil {
		delete(vc.svcs, key)
		svcsGauge.Dec()
	}
	return err
}

// Update a Varnish Service to implement an Ingress.
//
//    svcKey: namespace/name key for the Service
//    ingKey: namespace/name key for the Ingress
//    uid: UID field from the Ingress
//    spec: VCL spec corresponding to the Ingress definition
func (vc *VarnishController) Update(
	svcKey, ingKey, uid string, spec vcl.Spec) error {

	svc, exists := vc.svcs[svcKey]
	if !exists {
		svc = &varnishSvc{instances: make([]*varnishInst, 0)}
		vc.svcs[svcKey] = svc
		svcsGauge.Inc()
		vc.log.Infof("Added Varnish service definition %s for Ingress "+
			"%s uid=%s", svcKey, ingKey, uid)
	}
	svc.cfgLoaded = false
	if svc.spec == nil {
		svc.spec = &vclSpec{}
	}
	svc.spec.key = ingKey
	svc.spec.uid = uid
	svc.spec.spec = spec

	if len(svc.instances) == 0 {
		return fmt.Errorf("Ingress %s uid=%s: Currently no known "+
			"endpoints for Varnish service %s", ingKey, uid, svcKey)
	}
	return vc.updateVarnishSvc(svcKey)
}

// DeleteIngress is called for the Delete event on an Ingress, and
// syncs its effect for a Varnish Service.
//
//    svcKey: namespace/name key for the Varnish Service
//    ingKey: namespace/name key for the Ingress
//
// We currently only support one Ingress definition at a time for a
// Varnish Service, so deleting the Ingress means that we set Varnish
// instances to the not ready state.
func (vc *VarnishController) DeleteIngress(svcKey, ingKey string) error {
	svc, ok := vc.svcs[svcKey]
	if !ok {
		return fmt.Errorf("Delete Ingress %s: no known Varnish service",
			ingKey)
	}
	if svc.spec != nil && svc.spec.key != ingKey {
		return fmt.Errorf("Delete Ingress %s: Ingress %s is assigned "+
			"to Varnish service %s", ingKey, svc.spec.key, svcKey)
	}
	svc.spec = nil

	var errs VarnishAdmErrors
	for _, inst := range svc.instances {
		if err := vc.setCfgLabel(inst, notAvailCfg, readinessLabel,
			false); err != nil {

			admErr := VarnishAdmError{
				addr: inst.addr,
				err:  err,
			}
			errs = append(errs, admErr)
			continue
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// HasIngress returns true iff an Ingress definition is already loaded
// for a Varnish Service (so a new sync attempt is not necessary).
//
//    svcKey: namespace/name key for the Varnish Service
//    ingKey: namespace/name key for the Ingress
//    uid: UID field from the Ingress
//    spec: VCL specification derived from the Ingress
func (vc *VarnishController) HasIngress(svcKey, ingKey, uid string,
	spec vcl.Spec) bool {

	svc, ok := vc.svcs[svcKey]
	if !ok {
		return false
	}
	return svc.cfgLoaded &&
		svc.spec.key == ingKey &&
		svc.spec.uid == uid &&
		reflect.DeepEqual(svc.spec.spec.Canonical(), spec.Canonical())
}

// SetAdmSecret stores the Secret data identified by the
// namespace/name key.
func (vc *VarnishController) SetAdmSecret(key string, secret []byte) {
	secr, exists := vc.secrets[key]
	if !exists {
		secretSlice := make([]byte, len(secret))
		secr = &secretSlice
		vc.secrets[key] = secr
		secretsGauge.Inc()
	}
	copy(*vc.secrets[key], secret)
}

// UpdateSvcForSecret associates the Secret identified by the
// namespace/name secretKey with the Varnish Service identified by the
// namespace/name svcKey. The Service is newly synced if necessary.
func (vc *VarnishController) UpdateSvcForSecret(svcKey, secretKey string) error {
	secret, exists := vc.secrets[secretKey]
	if !exists {
		secretKey = ""
		secret = nil
	}
	svc, exists := vc.svcs[svcKey]
	if !exists {
		if secret == nil {
			vc.log.Infof("Neither Varnish Service %s nor secret "+
				"%s found", svcKey, secretKey)
			return nil
		}
		vc.log.Infof("Creating Varnish Service %s to set secret %s",
			svcKey, secretKey)
		svc = &varnishSvc{instances: make([]*varnishInst, 0)}
		vc.svcs[svcKey] = svc
		svcsGauge.Inc()
	}
	svc.secrName = secretKey

	for _, inst := range svc.instances {
		vc.log.Infof("Setting secret for instance %s", inst.addr)
		inst.admSecret = secret
	}

	vc.log.Infof("Updating Service %s after setting secret %s", svcKey,
		secretKey)
	return vc.updateVarnishSvc(svcKey)
}

// DeleteAdmSecret removes the secret identified by the namespace/name
// key.
func (vc *VarnishController) DeleteAdmSecret(name string) {
	_, exists := vc.secrets[name]
	if exists {
		delete(vc.secrets, name)
		secretsGauge.Dec()
	}
}

// Quit stops the Varnish controller.
func (vc *VarnishController) Quit() {
	vc.errChan <- nil
}
