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

package controller

import (
	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	core_v1_listers "k8s.io/client-go/listers/core/v1"
	ext_listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/sirupsen/logrus"

	"code.uplex.de/uplex-varnish/k8s-ingress/cmd/varnish"
	ving_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	vcr_listers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/listers/varnishingress/v1alpha1"
)

const (
	// syncSuccess and syncFailure are reasons for Events
	syncSuccess = "SyncSuccess"
	syncFailure = "SyncFailure"
)

type NamespaceWorker struct {
	namespace   string
	log         *logrus.Logger
	vController *varnish.VarnishController
	queue       workqueue.RateLimitingInterface
	stopChan    chan struct{}
	ing         ext_listers.IngressNamespaceLister
	svc         core_v1_listers.ServiceNamespaceLister
	endp        core_v1_listers.EndpointsNamespaceLister
	secr        core_v1_listers.SecretNamespaceLister
	vcfg        vcr_listers.VarnishConfigNamespaceLister
	client      kubernetes.Interface
	recorder    record.EventRecorder
}

func (worker *NamespaceWorker) infoEvent(obj interface{}, reason, msgFmt string,
	args ...interface{}) {

	switch obj.(type) {
	case *extensions.Ingress:
		ing, _ := obj.(*extensions.Ingress)
		worker.recorder.Eventf(ing, api_v1.EventTypeNormal, reason,
			msgFmt, args...)
	case *api_v1.Service:
		svc, _ := obj.(*api_v1.Service)
		worker.recorder.Eventf(svc, api_v1.EventTypeNormal, reason,
			msgFmt, args...)
	case *api_v1.Endpoints:
		endp, _ := obj.(*api_v1.Endpoints)
		worker.recorder.Eventf(endp, api_v1.EventTypeNormal, reason,
			msgFmt, args...)
	case *api_v1.Secret:
		secr, _ := obj.(*api_v1.Secret)
		worker.recorder.Eventf(secr, api_v1.EventTypeNormal, reason,
			msgFmt, args...)
	case *ving_v1alpha1.VarnishConfig:
		vcfg, _ := obj.(*ving_v1alpha1.VarnishConfig)
		worker.recorder.Eventf(vcfg, api_v1.EventTypeNormal, reason,
			msgFmt, args...)
	default:
		worker.log.Warnf("Unhandled type %T, no event generated", obj)
	}
}

func (worker *NamespaceWorker) warnEvent(obj interface{}, reason, msgFmt string,
	args ...interface{}) {

	switch obj.(type) {
	case *extensions.Ingress:
		ing, _ := obj.(*extensions.Ingress)
		worker.recorder.Eventf(ing, api_v1.EventTypeWarning, reason,
			msgFmt, args...)
	case *api_v1.Service:
		svc, _ := obj.(*api_v1.Service)
		worker.recorder.Eventf(svc, api_v1.EventTypeWarning, reason,
			msgFmt, args...)
	case *api_v1.Endpoints:
		endp, _ := obj.(*api_v1.Endpoints)
		worker.recorder.Eventf(endp, api_v1.EventTypeWarning, reason,
			msgFmt, args...)
	case *api_v1.Secret:
		secr, _ := obj.(*api_v1.Secret)
		worker.recorder.Eventf(secr, api_v1.EventTypeWarning, reason,
			msgFmt, args...)
	case *ving_v1alpha1.VarnishConfig:
		vcfg, _ := obj.(*ving_v1alpha1.VarnishConfig)
		worker.recorder.Eventf(vcfg, api_v1.EventTypeWarning, reason,
			msgFmt, args...)
	default:
		worker.log.Warnf("Unhandled type %T, no event generated", obj)
	}
}

func (worker *NamespaceWorker) syncSuccess(obj interface{}, msgFmt string,
	args ...interface{}) {

	worker.log.Infof(msgFmt, args...)
	worker.infoEvent(obj, syncSuccess, msgFmt, args...)
}

func (worker *NamespaceWorker) syncFailure(obj interface{}, msgFmt string,
	args ...interface{}) {

	worker.log.Errorf(msgFmt, args...)
	worker.warnEvent(obj, syncFailure, msgFmt, args...)
}

func (worker *NamespaceWorker) dispatch(obj interface{}) error {
	_, key, err := getNameSpace(obj)
	if err != nil {
		worker.syncFailure(obj, "Cannot get key for object %v: %v", obj,
			err)
		return nil
	}
	switch obj.(type) {
	case *extensions.Ingress:
		return worker.syncIng(key)
	case *api_v1.Service:
		return worker.syncSvc(key)
	case *api_v1.Endpoints:
		return worker.syncEndp(key)
	case *api_v1.Secret:
		return worker.syncSecret(key)
	case *ving_v1alpha1.VarnishConfig:
		return worker.syncVcfg(key)
	default:
		deleted, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			worker.syncFailure(obj, "Unhandled object type: %T",
				obj)
			return nil
		}
		switch deleted.Obj.(type) {
		case *extensions.Ingress:
			return worker.deleteIng(key)
		case *api_v1.Service:
			return worker.deleteSvc(key)
		case *api_v1.Endpoints:
			// Delete and sync do the same thing
			return worker.syncEndp(key)
		case *api_v1.Secret:
			return worker.deleteSecret(key)
		case *ving_v1alpha1.VarnishConfig:
			return worker.deleteVcfg(key)
		default:
			worker.syncFailure(deleted, "Unhandled object type: %T",
				deleted)
			return nil
		}
		return nil
	}
}

func (worker *NamespaceWorker) next() {
	select {
	case <-worker.stopChan:
		worker.queue.ShutDown()
		return
	default:
		break
	}

	obj, quit := worker.queue.Get()
	if quit {
		return
	}
	defer worker.queue.Done(obj)

	if err := worker.dispatch(obj); err == nil {
		if ns, name, err := getNameSpace(obj); err == nil {
			worker.syncSuccess(obj, "Successfully synced: %s/%s",
				ns, name)
		} else {
			worker.syncSuccess(obj, "Successfully synced")
		}
		worker.queue.Forget(obj)
	} else {
		worker.syncFailure(obj, "Error, requeueing: %v", err)
		worker.queue.AddRateLimited(obj)
	}
}

func (worker *NamespaceWorker) work() {
	worker.log.Info("Starting worker for namespace:", worker.namespace)

	for !worker.queue.ShuttingDown() {
		worker.next()
	}

	worker.log.Info("Shutting down worker for namespace:", worker.namespace)
}

type NamespaceQueues struct {
	Queue       workqueue.RateLimitingInterface
	log         *logrus.Logger
	vController *varnish.VarnishController
	workers     map[string]*NamespaceWorker
	listers     *Listers
	client      kubernetes.Interface
	recorder    record.EventRecorder
}

func NewNamespaceQueues(
	log *logrus.Logger,
	vController *varnish.VarnishController,
	listers *Listers,
	client kubernetes.Interface,
	recorder record.EventRecorder) *NamespaceQueues {

	q := workqueue.NewRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter())
	return &NamespaceQueues{
		Queue:       q,
		log:         log,
		vController: vController,
		workers:     make(map[string]*NamespaceWorker),
		listers:     listers,
		client:      client,
		recorder:    recorder,
	}
}

func getNameSpace(obj interface{}) (namespace, name string, err error) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	namespace, name, err = cache.SplitMetaNamespaceKey(k)
	if err != nil {
		return
	}
	return
}

func (qs *NamespaceQueues) next() {
	obj, quit := qs.Queue.Get()
	if quit {
		return
	}
	defer qs.Queue.Done(obj)

	ns, _, err := getNameSpace(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	worker, exists := qs.workers[ns]
	if !exists {
		q := workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), ns)
		worker = &NamespaceWorker{
			namespace:   ns,
			log:         qs.log,
			vController: qs.vController,
			queue:       q,
			stopChan:    make(chan struct{}),
			ing:         qs.listers.ing.Ingresses(ns),
			svc:         qs.listers.svc.Services(ns),
			endp:        qs.listers.endp.Endpoints(ns),
			secr:        qs.listers.secr.Secrets(ns),
			vcfg:        qs.listers.vcfg.VarnishConfigs(ns),
			client:      qs.client,
			recorder:    qs.recorder,
		}
		qs.workers[ns] = worker
		go worker.work()
	}
	worker.queue.Add(obj)
	qs.Queue.Forget(obj)
}

func (qs *NamespaceQueues) Run() {
	qs.log.Info("Starting dispatcher worker")
	for !qs.Queue.ShuttingDown() {
		qs.next()
	}
	qs.log.Info("Shutting down dispatcher worker")
}

func (qs *NamespaceQueues) Stop() {
	qs.Queue.ShutDown()
	for _, worker := range qs.workers {
		worker.queue.ShutDown()
		close(worker.stopChan)
	}
	// XXX wait for WaitGroup
}
