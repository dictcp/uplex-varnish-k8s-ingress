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

package varnish

import (
	"strings"
	"time"

	"code.uplex.de/uplex-varnish/varnishapi/pkg/admin"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	noAdmSecret  = "NoAdminSecret"
	connectErr   = "ConnectFailure"
	pingErr      = "PingFailure"
	statusErr    = "StatusFailure"
	statusNotRun = "StatusNotRunning"
	panicErr     = "PanicFailure"
	panic        = "Panic"
	vclListErr   = "VCLListFailure"
	discardErr   = "VCLDiscardFailure"
	updateErr    = "UpdateFailure"
	monitorGood  = "MonitorGood"
)

func (vc *VarnishController) infoEvt(svc, reason, msgFmt string,
	args ...interface{}) {

	vc.log.Infof(msgFmt, args...)
	vc.svcEvt.SvcInfoEvent(svc, reason, msgFmt, args...)
	monResultCtr.WithLabelValues(svc, "info", reason).Inc()
}

func (vc *VarnishController) warnEvt(svc, reason, msgFmt string,
	args ...interface{}) {

	vc.log.Warnf(msgFmt, args...)
	vc.svcEvt.SvcWarnEvent(svc, reason, msgFmt, args...)
	monResultCtr.WithLabelValues(svc, "warning", reason).Inc()
}

func (vc *VarnishController) errorEvt(svc, reason, msgFmt string,
	args ...interface{}) {

	vc.log.Errorf(msgFmt, args...)
	vc.svcEvt.SvcWarnEvent(svc, reason, msgFmt, args...)
	monResultCtr.WithLabelValues(svc, "error", reason).Inc()
}

func (vc *VarnishController) checkInst(svc string, inst *varnishInst) bool {
	metrics := getInstanceMetrics(inst.addr)
	metrics.monitorChecks.Inc()

	vc.log.Infof("Monitoring Varnish instance %s (Service %s)", inst.addr,
		svc)

	if inst.admSecret == nil {
		vc.warnEvt(svc, noAdmSecret,
			"No admin secret known for endpoint %s", inst.addr)
		return false
	}
	inst.admMtx.Lock()
	defer inst.admMtx.Unlock()
	vc.wg.Add(1)
	defer vc.wg.Done()

	timer := prometheus.NewTimer(metrics.connectLatency)
	adm, err := admin.Dial(inst.addr, *inst.admSecret, admTimeout)
	timer.ObserveDuration()
	if err != nil {
		metrics.connectFails.Inc()
		vc.errorEvt(svc, connectErr, "Error connecting to %s: %v",
			inst.addr, err)
		return false
	}
	defer adm.Close()
	inst.Banner = adm.Banner
	vc.log.Debugf("Connected to Varnish instance %s, banner: %s", inst.addr,
		adm.Banner)

	pong, err := adm.Ping()
	if err != nil {
		metrics.pingFails.Inc()
		vc.errorEvt(svc, pingErr, "Error pinging at %s: %v", inst.addr,
			err)
		return false
	}
	metrics.pings.Inc()
	vc.log.Debugf("Succesfully pinged instance %s: %+v", inst.addr, pong)

	state, err := adm.Status()
	if err != nil {
		vc.errorEvt(svc, statusErr, "Error getting status at %s: %v",
			inst.addr, err)
		return false
	}
	if state == admin.Running {
		metrics.childRunning.Inc()
		vc.log.Debugf("Status at %s: %s", inst.addr, state)
	} else {
		metrics.childNotRunning.Inc()
		vc.warnEvt(svc, statusNotRun, "Status at %s: %s", inst.addr,
			state)
	}

	panic, err := adm.GetPanic()
	if err != nil {
		vc.errorEvt(svc, panicErr, "Error getting panic at %s: %v",
			inst.addr, err)
		return false
	}
	if panic == "" {
		vc.log.Debugf("No panic at %s", inst.addr)
	} else {
		metrics.panics.Inc()
		vc.errorEvt(svc, panic, "Panic at %s: %s", inst.addr, panic)
		// XXX clear the panic? Should be configurable
	}

	vcls, err := adm.VCLList()
	if err != nil {
		vc.errorEvt(svc, vclListErr,
			"Error getting VCL list at %s: %v", inst.addr, err)
		return false
	}
	for _, vcl := range vcls {
		if strings.HasPrefix(vcl.Name, ingressPrefix) &&
			vcl.State == admin.ColdState {
			if err = adm.VCLDiscard(vcl.Name); err != nil {
				vc.errorEvt(svc, discardErr,
					"Error discarding VCL %s at %s: "+
						"%v", vcl.Name, inst.addr, err)
				return false
			}
			metrics.vclDiscards.Inc()
			vc.log.Infof("Discarded VCL %s at %s", vcl.Name,
				inst.addr)
		}
	}
	return true
}

func (vc *VarnishController) monitor(monitorIntvl time.Duration) {
	if monitorIntvl <= 0 {
		vc.log.Infof("Varnish monitor interval=%v, monitor not running",
			monitorIntvl)
		return
	}
	vc.log.Info("Varnish monitor starting, interval: ", monitorIntvl)

	for {
		time.Sleep(monitorIntvl)

		for svcName, svc := range vc.svcs {
			vc.log.Infof("Monitoring Varnish instances in %s",
				svcName)

			good := true
			for _, inst := range svc.instances {
				if !vc.checkInst(svcName, inst) {
					good = false
				}
			}

			if err := vc.updateVarnishSvc(svcName); err != nil {
				vc.errorEvt(svcName, updateErr,
					"Errors updating Varnish "+
						"Service %s: %+v", svcName, err)
				good = false
			}
			if good {
				vc.infoEvt(svcName, monitorGood,
					"Monitor check good for Service: %s",
					svcName)
			}
		}
	}
}
