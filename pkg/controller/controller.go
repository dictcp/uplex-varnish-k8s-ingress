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
	"os"
	"time"

	vcr_informers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/informers/externalversions"
	vcr_listers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/listers/varnishingress/v1alpha1"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	core_v1_listers "k8s.io/client-go/listers/core/v1"
	ext_listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type infrmrs struct {
	ing  cache.SharedIndexInformer
	svc  cache.SharedIndexInformer
	endp cache.SharedIndexInformer
	secr cache.SharedIndexInformer
	vcfg cache.SharedIndexInformer
}

// Listers aggregates listers from k8s.io/client-go/listers for the
// various resource types of interested. These are initialized by
// IngressController, and handed off to NamespaceWorker workers to
// read data from the client-go cache.
type Listers struct {
	ing  ext_listers.IngressLister
	svc  core_v1_listers.ServiceLister
	endp core_v1_listers.EndpointsLister
	secr core_v1_listers.SecretLister
	vcfg vcr_listers.VarnishConfigLister
}

// IngressController watches Kubernetes API and reconfigures Varnish
// via VarnishController when needed.
type IngressController struct {
	log         *logrus.Logger
	client      kubernetes.Interface
	vController *varnish.VarnishController
	informers   *infrmrs
	listers     *Listers
	nsQs        *NamespaceQueues
	stopCh      chan struct{}
	recorder    record.EventRecorder
}

// NewIngressController creates a controller.
//
//    log: logger initialized at startup
//    kubeClient: k8s client initialized at startup
//    vc: Varnish controller
//    infFactory: SharedInformerFactory to create informers & listers for
//                the k8s standard client APIs
//    vcrInfFactory: SharedInformerFactory for the project's own client APIs
func NewIngressController(
	log *logrus.Logger,
	kubeClient kubernetes.Interface,
	vc *varnish.VarnishController,
	infFactory informers.SharedInformerFactory,
	vcrInfFactory vcr_informers.SharedInformerFactory) *IngressController {

	ingc := IngressController{
		log:         log,
		client:      kubeClient,
		stopCh:      make(chan struct{}),
		vController: vc,
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(ingc.log.Printf)
	eventBroadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{
		Interface: ingc.client.CoreV1().Events(""),
		// Interface: core_v1.New(ingc.client.CoreV1().RESTClient()).
		// 	Events(""),
	})
	ingc.recorder = eventBroadcaster.NewRecorder(scheme.Scheme,
		api_v1.EventSource{Component: "varnish-ingress-controller"})

	ingc.informers = &infrmrs{
		ing:  infFactory.Extensions().V1beta1().Ingresses().Informer(),
		svc:  infFactory.Core().V1().Services().Informer(),
		endp: infFactory.Core().V1().Endpoints().Informer(),
		secr: infFactory.Core().V1().Secrets().Informer(),
		vcfg: vcrInfFactory.Ingress().V1alpha1().VarnishConfigs().
			Informer(),
	}

	evtFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc:    ingc.handleObj,
		DeleteFunc: ingc.handleObj,
		UpdateFunc: ingc.updateObj,
	}

	ingc.informers.ing.AddEventHandler(evtFuncs)
	ingc.informers.svc.AddEventHandler(evtFuncs)
	ingc.informers.endp.AddEventHandler(evtFuncs)
	ingc.informers.secr.AddEventHandler(evtFuncs)
	ingc.informers.vcfg.AddEventHandler(evtFuncs)

	ingc.listers = &Listers{
		ing:  infFactory.Extensions().V1beta1().Ingresses().Lister(),
		svc:  infFactory.Core().V1().Services().Lister(),
		endp: infFactory.Core().V1().Endpoints().Lister(),
		secr: infFactory.Core().V1().Secrets().Lister(),
		vcfg: vcrInfFactory.Ingress().V1alpha1().VarnishConfigs().
			Lister(),
	}

	ingc.nsQs = NewNamespaceQueues(ingc.log, ingc.vController, ingc.listers,
		ingc.client, ingc.recorder)

	return &ingc
}

func (ingc *IngressController) handleObj(obj interface{}) {
	ingc.log.Debug("Handle:", obj)
	m, mErr := meta.Accessor(obj)
	t, tErr := meta.TypeAccessor(obj)
	if mErr == nil && tErr == nil {
		ingc.log.Infof("Handle %s: %s/%s", t.GetKind(),
			m.GetNamespace(), m.GetName())
	}
	ingc.nsQs.Queue.Add(obj)
}

func (ingc *IngressController) updateObj(old, new interface{}) {
	ingc.log.Debug("Update:", old, new)
	oldMeta, oldErr := meta.Accessor(old)
	newMeta, newErr := meta.Accessor(new)
	t, tErr := meta.TypeAccessor(old)
	if oldErr == nil && newErr == nil &&
		oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
		if tErr == nil && t.GetKind() != "" {
			ingc.log.Infof("Update %s %s/%s: unchanged",
				t.GetKind(), oldMeta.GetNamespace(),
				oldMeta.GetName())
		} else {
			ingc.log.Infof("Update %s/%s: unchanged",
				oldMeta.GetNamespace(), oldMeta.GetName())
		}
		return
	}

	// kube-system resources frequently update Endpoints with
	// empty Subsets, ignore them.
	oldEndp, oldEndpExists := old.(*api_v1.Endpoints)
	newEndp, newEndpExists := new.(*api_v1.Endpoints)
	if oldEndpExists && newEndpExists &&
		len(oldEndp.Subsets) == 0 && len(newEndp.Subsets) == 0 {

		ingc.log.Infof("Update endpoints %s/%s: empty Subsets, ignoring",
			newEndp.Namespace, newEndp.Name)
		return
	}

	var metaObj *meta_v1.Object
	if oldErr == nil {
		metaObj = &oldMeta
	} else if newErr == nil {
		metaObj = &newMeta
	}
	if metaObj != nil {
		if tErr == nil && t.GetKind() != "" {
			ingc.log.Infof("Update %s: %s/%s", t.GetKind(),
				(*metaObj).GetNamespace(), (*metaObj).GetName())
		} else {
			ingc.log.Infof("Update: %s/%s",
				(*metaObj).GetNamespace(), (*metaObj).GetName())
		}
	}
	ingc.nsQs.Queue.Add(new)
}

// Run the Ingress controller -- start the informers in goroutines,
// wait for the caches to sync, and call Run() for the
// NamespaceQueues. Then block until Stop() is invoked.
//
// If readyFile is non-empty, it is the path of a file to touch when
// the controller is ready (after informers have launched).
func (ingc *IngressController) Run(readyFile string) {
	defer utilruntime.HandleCrash()
	defer ingc.nsQs.Stop()

	ingc.log.Info("Launching informers")
	go ingc.informers.ing.Run(ingc.stopCh)
	go ingc.informers.svc.Run(ingc.stopCh)
	go ingc.informers.endp.Run(ingc.stopCh)
	go ingc.informers.secr.Run(ingc.stopCh)
	go ingc.informers.vcfg.Run(ingc.stopCh)

	ingc.log.Info("Controller ready")
	if readyFile != "" {
		f, err := os.Create(readyFile)
		if err != nil {
			e := fmt.Errorf("Cannot create ready file %s: %v",
				readyFile, err)
			utilruntime.HandleError(e)
			return
		}
		if err = f.Close(); err != nil {
			e := fmt.Errorf("Cannot close ready file %s: %v",
				readyFile, err)
			utilruntime.HandleError(e)
			defer f.Close()
		}
		ingc.log.Infof("Created ready file %s", readyFile)
	}

	ingc.log.Info("Waiting for caches to sync")
	if ok := cache.WaitForCacheSync(ingc.stopCh,
		ingc.informers.ing.HasSynced,
		ingc.informers.svc.HasSynced,
		ingc.informers.endp.HasSynced,
		ingc.informers.secr.HasSynced,
		ingc.informers.vcfg.HasSynced); !ok {

		err := fmt.Errorf("Failed waiting for caches to sync")
		utilruntime.HandleError(err)
		return
	}

	ingc.log.Info("Caches synced, running workers")
	go wait.Until(ingc.nsQs.Run, time.Second, ingc.stopCh)

	<-ingc.stopCh
	ingc.log.Info("Shutting down workers")
}

// Stop the Ingress controller -- signal the workers to stop.
func (ingc *IngressController) Stop() {
	close(ingc.stopCh)
}
