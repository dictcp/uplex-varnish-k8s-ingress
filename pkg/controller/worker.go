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
	"fmt"
	"sync"

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

	ving_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	vcr_listers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/listers/varnishingress/v1alpha1"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish"
)

const (
	// syncSuccess and syncFailure are reasons for Events
	syncSuccess = "SyncSuccess"
	syncFailure = "SyncFailure"
)

// NamespaceWorker serves fanout of work items to workers for each
// namespace for which the controller is notified about a resource
// update. The NamespaceQueues object creates a new instance when it
// reads an item from a new namespace from its main queue. Each worker
// has its own queue and listers filtered for its namespace. Thus each
// namespace is synced separately and sequentially, and all of the
// namespaces are synced in parallel.
type NamespaceWorker struct {
	namespace   string
	ingClass    string
	log         *logrus.Logger
	vController *varnish.VarnishController
	queue       workqueue.RateLimitingInterface
	listers     *Listers
	ing         ext_listers.IngressNamespaceLister
	svc         core_v1_listers.ServiceNamespaceLister
	endp        core_v1_listers.EndpointsNamespaceLister
	secr        core_v1_listers.SecretNamespaceLister
	vcfg        vcr_listers.VarnishConfigNamespaceLister
	bcfg        vcr_listers.BackendConfigNamespaceLister
	client      kubernetes.Interface
	recorder    record.EventRecorder
	wg          *sync.WaitGroup
}

func (worker *NamespaceWorker) event(obj interface{}, evtType, reason,
	msgFmt string, args ...interface{}) {

	eventObj := obj
	if syncObj, ok := obj.(*SyncObj); ok {
		eventObj = syncObj.Obj
	}
	kind := "Unknown"
	switch eventObj.(type) {
	case *extensions.Ingress:
		ing, _ := eventObj.(*extensions.Ingress)
		worker.recorder.Eventf(ing, evtType, reason, msgFmt, args...)
		kind = "Ingress"
	case *api_v1.Service:
		svc, _ := eventObj.(*api_v1.Service)
		worker.recorder.Eventf(svc, evtType, reason, msgFmt, args...)
		kind = "Service"
	case *api_v1.Endpoints:
		endp, _ := eventObj.(*api_v1.Endpoints)
		worker.recorder.Eventf(endp, evtType, reason, msgFmt, args...)
		kind = "Endpoints"
	case *api_v1.Secret:
		secr, _ := eventObj.(*api_v1.Secret)
		worker.recorder.Eventf(secr, evtType, reason, msgFmt, args...)
		kind = "Secret"
	case *ving_v1alpha1.VarnishConfig:
		vcfg, _ := eventObj.(*ving_v1alpha1.VarnishConfig)
		worker.recorder.Eventf(vcfg, evtType, reason, msgFmt, args...)
		kind = "VarnishConfig"
	case *ving_v1alpha1.BackendConfig:
		bcfg, _ := eventObj.(*ving_v1alpha1.BackendConfig)
		worker.recorder.Eventf(bcfg, evtType, reason, msgFmt, args...)
		kind = "BackendConfig"
	default:
		worker.log.Warnf("Unhandled type %T, no event generated",
			eventObj)
	}
	syncCounters.WithLabelValues(worker.namespace, kind, reason).Inc()
}

func (worker *NamespaceWorker) infoEvent(obj interface{}, reason, msgFmt string,
	args ...interface{}) {

	worker.event(obj, api_v1.EventTypeNormal, reason, msgFmt, args...)
}

func (worker *NamespaceWorker) warnEvent(obj interface{}, reason, msgFmt string,
	args ...interface{}) {

	worker.event(obj, api_v1.EventTypeWarning, reason, msgFmt, args...)
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
	syncObj, ok := obj.(*SyncObj)
	if !ok {
		worker.syncFailure(obj, "Unhandled type %T", obj)
		return nil
	}
	_, key, err := getNameSpace(syncObj.Obj)
	if err != nil {
		worker.syncFailure(syncObj.Obj,
			"Cannot get key for object %v: %v", syncObj.Obj, err)
		return nil
	}
	switch syncObj.Type {
	case Add:
		switch syncObj.Obj.(type) {
		case *extensions.Ingress:
			return worker.addIng(key)
		case *api_v1.Service:
			return worker.addSvc(key)
		case *api_v1.Endpoints:
			return worker.addEndp(key)
		case *api_v1.Secret:
			return worker.addSecret(key)
		case *ving_v1alpha1.VarnishConfig:
			return worker.addVcfg(key)
		case *ving_v1alpha1.BackendConfig:
			return worker.addBcfg(key)
		default:
			worker.syncFailure(syncObj.Obj,
				"Unhandled object type: %T", syncObj.Obj)
			return nil
		}
	case Update:
		switch syncObj.Obj.(type) {
		case *extensions.Ingress:
			return worker.updateIng(key)
		case *api_v1.Service:
			return worker.updateSvc(key)
		case *api_v1.Endpoints:
			return worker.updateEndp(key)
		case *api_v1.Secret:
			return worker.updateSecret(key)
		case *ving_v1alpha1.VarnishConfig:
			return worker.updateVcfg(key)
		case *ving_v1alpha1.BackendConfig:
			return worker.updateBcfg(key)
		default:
			worker.syncFailure(syncObj.Obj,
				"Unhandled object type: %T", syncObj.Obj)
			return nil
		}
	case Delete:
		deletedObj := syncObj.Obj
		deleted, ok := obj.(cache.DeletedFinalStateUnknown)
		if ok {
			deletedObj = deleted.Obj
		}
		switch deletedObj.(type) {
		case *extensions.Ingress:
			return worker.deleteIng(deletedObj)
		case *api_v1.Service:
			return worker.deleteSvc(deletedObj)
		case *api_v1.Endpoints:
			return worker.deleteEndp(deletedObj)
		case *api_v1.Secret:
			return worker.deleteSecret(deletedObj)
		case *ving_v1alpha1.VarnishConfig:
			return worker.deleteVcfg(deletedObj)
		case *ving_v1alpha1.BackendConfig:
			return worker.deleteBcfg(deletedObj)
		default:
			worker.syncFailure(deletedObj,
				"Unhandled object type: %T", deletedObj)
			return nil
		}
	default:
		worker.syncFailure(syncObj.Obj, "Unhandled sync type: %v",
			syncObj.Type)
		return nil
	}
}

func (worker *NamespaceWorker) next() {
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
	defer worker.wg.Done()

	worker.log.Info("Starting worker for namespace:", worker.namespace)

	for !worker.queue.ShuttingDown() {
		worker.next()
	}

	worker.log.Info("Shutting down worker for namespace:", worker.namespace)
}

// NamespaceQueues reads from the main queue to which informers add
// new work items from all namespaces. The worker reads items from the
// queue and places them on separate queues for NamespaceWorkers
// responsible for each namespace.
type NamespaceQueues struct {
	Queue       workqueue.RateLimitingInterface
	DoneChan    chan struct{}
	ingClass    string
	log         *logrus.Logger
	vController *varnish.VarnishController
	workers     map[string]*NamespaceWorker
	listers     *Listers
	client      kubernetes.Interface
	recorder    record.EventRecorder
	wg          *sync.WaitGroup
}

// NewNamespaceQueues creates a NamespaceQueues object.
//
//    log: logger initialized at startup
//    ingClass: value of the ingress.class Ingress annotation
//    vController: Varnish controller initialied at startup
//    listers: client-go/lister instance for each resource type
//    client: k8s API client initialized at startup
//    recorder: Event broadcaster initialized at startup
func NewNamespaceQueues(
	log *logrus.Logger,
	ingClass string,
	vController *varnish.VarnishController,
	listers *Listers,
	client kubernetes.Interface,
	recorder record.EventRecorder) *NamespaceQueues {

	q := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(), "_ALL_")
	return &NamespaceQueues{
		Queue:       q,
		DoneChan:    make(chan struct{}),
		log:         log,
		ingClass:    ingClass,
		vController: vController,
		workers:     make(map[string]*NamespaceWorker),
		listers:     listers,
		client:      client,
		recorder:    recorder,
		wg:          new(sync.WaitGroup),
	}
}

func getNameSpace(obj interface{}) (namespace, name string, err error) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	namespace, name, err = cache.SplitMetaNamespaceKey(k)
	return
}

func (qs *NamespaceQueues) next() {
	obj, quit := qs.Queue.Get()
	if quit {
		return
	}
	defer qs.Queue.Done(obj)

	syncObj, ok := obj.(*SyncObj)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("Unhandled type %T", obj))
		return
	}
	ns, _, err := getNameSpace(syncObj.Obj)
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
			ingClass:    qs.ingClass,
			log:         qs.log,
			vController: qs.vController,
			queue:       q,
			listers:     qs.listers,
			ing:         qs.listers.ing.Ingresses(ns),
			svc:         qs.listers.svc.Services(ns),
			endp:        qs.listers.endp.Endpoints(ns),
			secr:        qs.listers.secr.Secrets(ns),
			vcfg:        qs.listers.vcfg.VarnishConfigs(ns),
			bcfg:        qs.listers.bcfg.BackendConfigs(ns),
			client:      qs.client,
			recorder:    qs.recorder,
			wg:          qs.wg,
		}
		qs.workers[ns] = worker
		go worker.work()
		qs.wg.Add(1)
	}
	worker.queue.Add(obj)
	qs.Queue.Forget(obj)
}

// Run comprises the main loop of the controller, reading from the
// main queue of work items and handing them off to workers for each
// namespace.
func (qs *NamespaceQueues) Run() {
	qs.log.Info("Starting dispatcher worker")
	for !qs.Queue.ShuttingDown() {
		qs.next()
	}
}

// Stop shuts down the main queue loop initiated by Run(), and in turn
// shuts down all of the NamespaceWorkers.
func (qs *NamespaceQueues) Stop() {
	qs.log.Info("Shutting down dispatcher worker")
	qs.Queue.ShutDown()
	for _, worker := range qs.workers {
		qs.log.Infof("Shutting down queue for namespace: %s",
			worker.namespace)
		worker.queue.ShutDown()
	}
	qs.log.Info("Waiting for workers to shut down")
	qs.wg.Wait()
	close(qs.DoneChan)
}
