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
	"strings"
	"time"

	"code.uplex.de/uplex-varnish/varnishapi/pkg/admin"
)

const monitorIntvl = time.Second * 30

func (vc *VarnishController) checkInst(inst *varnishSvc) {
	inst.admMtx.Lock()
	defer inst.admMtx.Unlock()

	adm, err := admin.Dial(inst.addr, vc.admSecret, admTimeout)
	if err != nil {
		vc.log.Errorf("Error connecting to %s: %v", inst.addr, err)
		return
	}
	defer adm.Close()
	inst.Banner = adm.Banner
	vc.log.Infof("Connected to Varnish instance %s", inst.addr)

	pong, err := adm.Ping()
	if err != nil {
		vc.log.Error("Error pinging at %s: %v", inst.addr, err)
		return
	}
	vc.log.Infof("Succesfully pinged instance %s: %+v", inst.addr, pong)

	state, err := adm.Status()
	if err != nil {
		vc.log.Error("Error getting status at %s: %v", inst.addr, err)
		return
	}
	vc.log.Infof("Status at %s: %s", inst.addr, state)

	panic, err := adm.GetPanic()
	if err != nil {
		vc.log.Error("Error getting panic at %s: %v", inst.addr, err)
		return
	}
	if panic == "" {
		vc.log.Infof("No panic at %s", inst.addr)
	} else {
		vc.log.Warnf("Panic at %s: %s", inst.addr, panic)
		// XXX clear the panic? Should be configurable
	}

	vcls, err := adm.VCLList()
	if err != nil {
		vc.log.Error("Error getting VCL list at %s: %v", inst.addr, err)
		return
	}
	for _, vcl := range vcls {
		if strings.HasPrefix(vcl.Name, ingressPrefix) &&
			vcl.State == admin.ColdState {
			if err = adm.VCLDiscard(vcl.Name); err != nil {
				vc.log.Errorf("Error discarding VCL %s at %s: "+
					"%v", vcl.Name, inst.addr, err)
				return
			}
			vc.log.Infof("Discarded VCL %s at %s", vcl.Name,
				inst.addr)
		}
	}
}

func (vc *VarnishController) monitor() {
	vc.log.Info("Varnish monitor starting")

	for {
		time.Sleep(monitorIntvl)

		for svc, insts := range vc.varnishSvcs {
			vc.log.Infof("Monitoring Varnish instances in %s", svc)

			for _, inst := range insts {
				vc.checkInst(inst)
			}

			if err := vc.updateVarnishInstances(insts); err != nil {
				vc.log.Errorf("Errors updating Varnish "+
					"instances: %+v", err)
			}
		}
	}
}
