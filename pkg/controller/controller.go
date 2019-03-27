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

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	vcr_informers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/informers/externalversions"
	vcr_listers "code.uplex.de/uplex-varnish/k8s-ingress/pkg/client/listers/varnishingress/v1alpha1"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	core_v1_listers "k8s.io/client-go/listers/core/v1"
	ext_listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type infrmrs struct {
	ing  cache.SharedIndexInformer
	svc  cache.SharedIndexInformer
	endp cache.SharedIndexInformer
	secr cache.SharedIndexInformer
	vcfg cache.SharedIndexInformer
	bcfg cache.SharedIndexInformer
}

// SyncType classifies the sync event, passed through to workers.
type SyncType uint8

const (
	// Add event
	Add SyncType = iota
	// Update event
	Update
	// Delete event
	Delete
)

// SyncObj wraps the object for which event handlers are notified, and
// encodes the sync event. These are the objects passed into the
// queues for workers.
type SyncObj struct {
	Type SyncType
	Obj  interface{}
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
	bcfg vcr_listers.BackendConfigLister
}

// IngressController watches Kubernetes API and reconfigures Varnish
// via varnish.Controller when needed.
type IngressController struct {
	log         *logrus.Logger
	client      kubernetes.Interface
	vController *varnish.Controller
	informers   *infrmrs
	listers     *Listers
	nsQs        *NamespaceQueues
	stopCh      chan struct{}
	recorder    record.EventRecorder
}

// NewIngressController creates a controller.
//
//    log: logger initialized at startup
//    ingClass: value of the ingress.class Ingress annotation
//    kubeClient: k8s client initialized at startup
//    vc: Varnish controller
//    infFactory: SharedInformerFactory to create informers & listers for
//                the k8s standard client APIs
//    vcrInfFactory: SharedInformerFactory for the project's own client APIs
func NewIngressController(
	log *logrus.Logger,
	ingClass string,
	kubeClient kubernetes.Interface,
	vc *varnish.Controller,
	infFactory informers.SharedInformerFactory,
	vcrInfFactory vcr_informers.SharedInformerFactory,
) (*IngressController, error) {

	ingc := IngressController{
		log:         log,
		client:      kubeClient,
		stopCh:      make(chan struct{}),
		vController: vc,
	}

	InitMetrics()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(ingc.log.Printf)
	eventBroadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{
		Interface: ingc.client.CoreV1().Events(""),
	})
	evtScheme := runtime.NewScheme()
	if err := api_v1.AddToScheme(evtScheme); err != nil {
		return nil, err
	}
	if err := extensions.AddToScheme(evtScheme); err != nil {
		return nil, err
	}
	if err := vcr_v1alpha1.AddToScheme(evtScheme); err != nil {
		return nil, err
	}
	ingc.recorder = eventBroadcaster.NewRecorder(evtScheme,
		api_v1.EventSource{Component: "varnish-ingress-controller"})

	ingc.informers = &infrmrs{
		ing:  infFactory.Extensions().V1beta1().Ingresses().Informer(),
		svc:  infFactory.Core().V1().Services().Informer(),
		endp: infFactory.Core().V1().Endpoints().Informer(),
		secr: infFactory.Core().V1().Secrets().Informer(),
		vcfg: vcrInfFactory.Ingress().V1alpha1().VarnishConfigs().
			Informer(),
		bcfg: vcrInfFactory.Ingress().V1alpha1().BackendConfigs().
			Informer(),
	}

	evtFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc:    ingc.addObj,
		DeleteFunc: ingc.deleteObj,
		UpdateFunc: ingc.updateObj,
	}

	ingc.informers.ing.AddEventHandler(evtFuncs)
	ingc.informers.svc.AddEventHandler(evtFuncs)
	ingc.informers.endp.AddEventHandler(evtFuncs)
	ingc.informers.secr.AddEventHandler(evtFuncs)
	ingc.informers.vcfg.AddEventHandler(evtFuncs)
	ingc.informers.bcfg.AddEventHandler(evtFuncs)

	ingc.listers = &Listers{
		ing:  infFactory.Extensions().V1beta1().Ingresses().Lister(),
		svc:  infFactory.Core().V1().Services().Lister(),
		endp: infFactory.Core().V1().Endpoints().Lister(),
		secr: infFactory.Core().V1().Secrets().Lister(),
		vcfg: vcrInfFactory.Ingress().V1alpha1().VarnishConfigs().
			Lister(),
		bcfg: vcrInfFactory.Ingress().V1alpha1().BackendConfigs().
			Lister(),
	}

	ingc.nsQs = NewNamespaceQueues(ingc.log, ingClass, ingc.vController,
		ingc.listers, ingc.client, ingc.recorder)

	return &ingc, nil
}

func (ingc *IngressController) logObj(action string, obj interface{}) {
	ingc.log.Debug(action, ":", obj)
	m, mErr := meta.Accessor(obj)
	t, tErr := meta.TypeAccessor(obj)
	if mErr == nil && tErr == nil {
		if t.GetKind() != "" {
			ingc.log.Debugf("%s %s: %s/%s", action, t.GetKind(),
				m.GetNamespace(), m.GetName())
		} else {
			ingc.log.Debugf("%s: %s/%s", action, m.GetNamespace(),
				m.GetName())
		}
	}
}

func incWatchCounter(obj interface{}, sync string) {
	switch obj.(type) {
	case *extensions.Ingress:
		watchCounters.WithLabelValues("Ingress", sync).Inc()
	case *api_v1.Service:
		watchCounters.WithLabelValues("Service", sync).Inc()
	case *api_v1.Endpoints:
		watchCounters.WithLabelValues("Endpoints", sync).Inc()
	case *api_v1.Secret:
		watchCounters.WithLabelValues("Secret", sync).Inc()
	case *vcr_v1alpha1.VarnishConfig:
		watchCounters.WithLabelValues("VarnishConfig", sync).Inc()
	case *vcr_v1alpha1.BackendConfig:
		watchCounters.WithLabelValues("BackendConfig", sync).Inc()
	default:
		watchCounters.WithLabelValues("Unknown", sync).Inc()
	}
}

func (ingc *IngressController) addObj(obj interface{}) {
	ingc.logObj("Add", obj)
	incWatchCounter(obj, "Add")
	ingc.nsQs.Queue.Add(&SyncObj{Type: Add, Obj: obj})
}

func (ingc *IngressController) deleteObj(obj interface{}) {
	ingc.logObj("Delete", obj)
	incWatchCounter(obj, "Delete")
	ingc.nsQs.Queue.Add(&SyncObj{Type: Delete, Obj: obj})
}

func (ingc *IngressController) updateObj(old, new interface{}) {
	ingc.log.Debug("Update:", old, new)
	incWatchCounter(new, "Update")
	oldMeta, oldErr := meta.Accessor(old)
	newMeta, newErr := meta.Accessor(new)
	t, tErr := meta.TypeAccessor(old)
	if oldErr == nil && newErr == nil &&
		oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() {
		if tErr == nil && t.GetKind() != "" {
			ingc.log.Debugf("Update %s %s/%s: unchanged",
				t.GetKind(), oldMeta.GetNamespace(),
				oldMeta.GetName())
			syncCounters.WithLabelValues(oldMeta.GetNamespace(),
				t.GetKind(), "Ignore").Inc()
		} else {
			kind := "Unknown"
			switch old.(type) {
			case *extensions.Ingress:
				kind = "Ingress"
			case *api_v1.Service:
				kind = "Service"
			case *api_v1.Endpoints:
				kind = "Endpoints"
			case *api_v1.Secret:
				kind = "Secret"
			case *vcr_v1alpha1.VarnishConfig:
				kind = "VarnishConfig"
			case *vcr_v1alpha1.BackendConfig:
				kind = "BackendConfig"
			}
			ingc.log.Debugf("Update %s %s/%s: unchanged", kind,
				oldMeta.GetNamespace(), oldMeta.GetName())
			syncCounters.WithLabelValues(oldMeta.GetNamespace(),
				kind, "Ignore").Inc()
		}
		return
	}

	// kube-system resources frequently update Endpoints with
	// empty Subsets, ignore them.
	oldEndp, oldEndpExists := old.(*api_v1.Endpoints)
	newEndp, newEndpExists := new.(*api_v1.Endpoints)
	if oldEndpExists && newEndpExists &&
		len(oldEndp.Subsets) == 0 && len(newEndp.Subsets) == 0 {

		ingc.log.Debugf("Update endpoints %s/%s: empty Subsets, ignoring",
			newEndp.Namespace, newEndp.Name)
		syncCounters.WithLabelValues(oldMeta.GetNamespace(),
			"Endpoints", "Ignore").Inc()
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
			ingc.log.Debugf("Update %s: %s/%s", t.GetKind(),
				(*metaObj).GetNamespace(), (*metaObj).GetName())
		} else {
			ingc.log.Debugf("Update: %s/%s",
				(*metaObj).GetNamespace(), (*metaObj).GetName())
		}
	}
	ingc.nsQs.Queue.Add(&SyncObj{Type: Update, Obj: new})
}

// Run the Ingress controller -- start the informers in goroutines,
// wait for the caches to sync, and call Run() for the
// NamespaceQueues. Then block until Stop() is invoked.
//
// If readyFile is non-empty, it is the path of a file to touch when
// the controller is ready (after informers have launched).
func (ingc *IngressController) Run(readyFile string, metricsPort uint16) {
	defer utilruntime.HandleCrash()
	defer ingc.nsQs.Stop()

	ingc.log.Info("Launching informers")
	go ingc.informers.ing.Run(ingc.stopCh)
	go ingc.informers.svc.Run(ingc.stopCh)
	go ingc.informers.endp.Run(ingc.stopCh)
	go ingc.informers.secr.Run(ingc.stopCh)
	go ingc.informers.vcfg.Run(ingc.stopCh)
	go ingc.informers.bcfg.Run(ingc.stopCh)

	ingc.log.Infof("Starting metrics listener at port %d", metricsPort)
	go ServeMetrics(ingc.log, metricsPort)

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
		ingc.informers.vcfg.HasSynced,
		ingc.informers.bcfg.HasSynced); !ok {

		err := fmt.Errorf("Failed waiting for caches to sync")
		utilruntime.HandleError(err)
		return
	}

	ingc.log.Info("Caches synced, running workers")
	go wait.Until(ingc.nsQs.Run, time.Second, ingc.stopCh)

	<-ingc.stopCh
}

// Stop the Ingress controller -- signal the workers to stop.
func (ingc *IngressController) Stop() {
	ingc.stopCh <- struct{}{}
	ingc.log.Info("Shutting down workers")
	close(ingc.stopCh)
	<-ingc.nsQs.DoneChan
	ingc.log.Info("Controller exiting")
}
