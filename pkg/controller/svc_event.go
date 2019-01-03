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

package controller

import (
	"fmt"

	api_v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (ingc *IngressController) svcEvent(svcKey, evtType, reason, msgFmt string,
	args ...interface{}) {

	namespace, name, err := cache.SplitMetaNamespaceKey(svcKey)
	if err != nil {
		e := fmt.Errorf("Cannot parse service key %s, will not "+
			"generate event(%s, %s): %v", svcKey, evtType, reason,
			err)
		utilruntime.HandleError(e)
		return
	}
	nsSvcs := ingc.listers.svc.Services(namespace)
	svc, err := nsSvcs.Get(name)
	if err != nil {
		e := fmt.Errorf("Cannot retrieve service %s/%s, will not "+
			"generate event(%s, %s): %v", namespace, name, evtType,
			reason, err)
		utilruntime.HandleError(e)
		return
	}
	ingc.recorder.Eventf(svc, evtType, reason, msgFmt, args...)
}

// SvcInfoEvent generates an Event with type "Normal" for the Service
// whose namespace/name is svcKey.
func (ingc *IngressController) SvcInfoEvent(svcKey, reason, msgFmt string,
	args ...interface{}) {

	ingc.svcEvent(svcKey, api_v1.EventTypeNormal, reason, msgFmt, args...)
}

// SvcWarnEvent generates an Event with type "Warning" for the Service
// whose namespace/name is svcKey.
func (ingc *IngressController) SvcWarnEvent(svcKey, reason, msgFmt string,
	args ...interface{}) {

	ingc.svcEvent(svcKey, api_v1.EventTypeWarning, reason, msgFmt, args...)
}
