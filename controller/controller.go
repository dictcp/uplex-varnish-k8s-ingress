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
	"log"
	"reflect"
	"time"

	"code.uplex.de/uplex-varnish/k8s-ingress/varnish"
	"code.uplex.de/uplex-varnish/k8s-ingress/varnish/vcl"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	scheme "k8s.io/client-go/kubernetes/scheme"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// XXX make these configurable
const (
	ingressClassKey = "kubernetes.io/ingress.class"
	resyncPeriod    = 0
	watchNamespace  = api_v1.NamespaceAll

//	resyncPeriod    = 30 * time.Second
)

// IngressController watches Kubernetes API and reconfigures Varnish
// via VarnishController when needed.
type IngressController struct {
	client         kubernetes.Interface
	vController    *varnish.VarnishController
	ingController  cache.Controller
	svcController  cache.Controller
	endpController cache.Controller
	ingLister      StoreToIngressLister
	svcLister      cache.Store
	endpLister     StoreToEndpointLister
	syncQueue      *taskQueue
	stopCh         chan struct{}
	recorder       record.EventRecorder
}

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// NewIngressController creates a controller
func NewIngressController(kubeClient kubernetes.Interface,
	vc *varnish.VarnishController, namespace string) *IngressController {

	ingc := IngressController{
		client:      kubeClient,
		stopCh:      make(chan struct{}),
		vController: vc,
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{
		Interface: core_v1.New(ingc.client.Core().RESTClient()).
			Events(""),
	})
	ingc.recorder = eventBroadcaster.NewRecorder(scheme.Scheme,
		api_v1.EventSource{Component: "varnish-ingress-controller"})

	ingc.syncQueue = NewTaskQueue(ingc.sync)

	log.Print("Varnish Ingress Controller has class: varnish")

	ingHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*extensions.Ingress)
			log.Print("ingHandler.AddFunc:", addIng)
			if !ingc.isVarnishIngress(addIng) {
				log.Printf("Ignoring Ingress %v based on "+
					"Annotation %v", addIng.Name,
					ingressClassKey)
				return
			}
			log.Printf("Adding Ingress: %v", addIng.Name)
			ingc.syncQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			remIng, isIng := obj.(*extensions.Ingress)
			log.Print("ingHandler.DeleteFunc:", remIng, isIng)
			if !isIng {
				deletedState, ok :=
					obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Print("Error received unexpected "+
						"object:", obj)
					return
				}
				remIng, ok =
					deletedState.Obj.(*extensions.Ingress)
				if !ok {
					log.Print("Error "+
						"DeletedFinalStateUnknown "+
						"contained non-Ingress object:",
						deletedState.Obj)
					return
				}
			}
			if !ingc.isVarnishIngress(remIng) {
				return
			}
			ingc.syncQueue.enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			curIng := cur.(*extensions.Ingress)
			oldIng := old.(*extensions.Ingress)
			log.Print("ingHandler.UpdateFunc:", curIng, oldIng)
			if !ingc.isVarnishIngress(curIng) {
				return
			}
			if hasChanges(oldIng, curIng) {
				log.Printf("Ingress %v changed, syncing",
					curIng.Name)
				ingc.syncQueue.enqueue(cur)
			}
		},
	}
	ingc.ingLister.Store, ingc.ingController = cache.NewInformer(
		cache.NewListWatchFromClient(ingc.client.Extensions().
			RESTClient(), "ingresses", namespace,
			fields.Everything()),
		&extensions.Ingress{}, resyncPeriod, ingHandlers)
	log.Print("ingc.ingLister.Store:", ingc.ingLister.Store)
	log.Print("ingc.ingController:", ingc.ingController)

	svcHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addSvc := obj.(*api_v1.Service)
			log.Print("svcHandler.AddFunc:", addSvc)
			if ingc.isExternalServiceForStatus(addSvc) {
				ingc.syncQueue.enqueue(addSvc)
				return
			}
			log.Print("Adding service:", addSvc.Name)
			ingc.enqueueIngressForService(addSvc)
		},
		DeleteFunc: func(obj interface{}) {
			remSvc, isSvc := obj.(*api_v1.Service)
			log.Print("svcHandler.DeleteFunc:", remSvc, isSvc)
			if !isSvc {
				deletedState, ok :=
					obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Print("Error received unexpected "+
						"object:", obj)
					return
				}
				remSvc, ok = deletedState.Obj.(*api_v1.Service)
				if !ok {
					log.Print("Error "+
						"DeletedFinalStateUnknown "+
						"contained non-Service object:",
						deletedState.Obj)
					return
				}
			}

			log.Print("Removing service:", remSvc.Name)
			ingc.enqueueIngressForService(remSvc)

		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				curSvc := cur.(*api_v1.Service)
				log.Print("svcHandler.UpdateFunc:", old, curSvc)
				if ingc.isExternalServiceForStatus(curSvc) {
					ingc.syncQueue.enqueue(curSvc)
					return
				}
				log.Printf("Service %v changed, syncing",
					curSvc.Name)
				ingc.enqueueIngressForService(curSvc)
			}
		},
	}
	ingc.svcLister, ingc.svcController = cache.NewInformer(
		cache.NewListWatchFromClient(ingc.client.Core().RESTClient(),
			"services", namespace, fields.Everything()),
		&api_v1.Service{}, resyncPeriod, svcHandlers)
	log.Print("ingc.svcLister.Store:", ingc.svcLister)
	log.Print("ingc.svcController:", ingc.svcController)

	endpHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addEndp := obj.(*api_v1.Endpoints)
			log.Print("endpHandler.UpdateFunc:", addEndp)
			log.Print("Adding endpoints:", addEndp.Name)
			ingc.syncQueue.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			remEndp, isEndp := obj.(*api_v1.Endpoints)
			log.Print("endpHandler.DeleteFunc:", remEndp, isEndp)
			if !isEndp {
				deletedState, ok :=
					obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Print("Error received unexpected "+
						"object:", obj)
					return
				}
				remEndp, ok =
					deletedState.Obj.(*api_v1.Endpoints)
				if !ok {
					log.Print("Error "+
						"DeletedFinalStateUnknown "+
						"contained non-Endpoints "+
						"object:", deletedState.Obj)
					return
				}
			}
			log.Print("Removing endpoints:", remEndp.Name)
			ingc.syncQueue.enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			log.Print("endpHandler.UpdateFunc:", old, cur)
			if !reflect.DeepEqual(old, cur) {
				log.Printf("Endpoints %v changed, syncing",
					cur.(*api_v1.Endpoints).Name)
				ingc.syncQueue.enqueue(cur)
			} else {
				log.Print("Update Endpoints: No change")
			}
		},
	}
	ingc.endpLister.Store, ingc.endpController = cache.NewInformer(
		cache.NewListWatchFromClient(ingc.client.Core().RESTClient(),
			"endpoints", namespace, fields.Everything()),
		&api_v1.Endpoints{}, resyncPeriod, endpHandlers)

	return &ingc
}

// hasChanges ignores Status or ResourceVersion changes
func hasChanges(oldIng *extensions.Ingress, curIng *extensions.Ingress) bool {
	oldIng.Status.LoadBalancer.Ingress = curIng.Status.LoadBalancer.Ingress
	oldIng.ResourceVersion = curIng.ResourceVersion
	return !reflect.DeepEqual(oldIng, curIng)
}

// Run starts the loadbalancer controller
func (ingc *IngressController) Run() {
	go ingc.svcController.Run(ingc.stopCh)
	go ingc.endpController.Run(ingc.stopCh)
	go ingc.ingController.Run(ingc.stopCh)
	go ingc.syncQueue.run(time.Second, ingc.stopCh)
	<-ingc.stopCh
}

// Stop shutdowns the load balancer controller
func (ingc *IngressController) Stop() {
	close(ingc.stopCh)

	ingc.syncQueue.shutdown()
}

func (ingc *IngressController) addOrUpdateIng(task Task,
	ing extensions.Ingress) {

	key := ing.ObjectMeta.Namespace + "/" + ing.ObjectMeta.Name
	log.Printf("Adding or Updating Ingress: %v\n", key)

	vclSpec, err := ingc.ing2VCLSpec(&ing)
	if err != nil {
		// XXX make the requeue interval configurable
		ingc.syncQueue.requeueAfter(task, err, 5*time.Second)
		ingc.recorder.Eventf(&ing, api_v1.EventTypeWarning, "Rejected",
			"%v was rejected: %v", key, err)
		return
	}

	err = ingc.vController.Update(key, vclSpec)
	if err != nil {
		// XXX as above
		ingc.syncQueue.requeueAfter(task, err, 5*time.Second)
		ingc.recorder.Eventf(&ing, api_v1.EventTypeWarning,
			"AddedOrUpdatedWithError",
			"Configuration for %v was added or updated, but not "+
				"applied: %v", key, err)
	}
}

func (ingc *IngressController) syncEndp(task Task) {
	key := task.Key
	log.Print("Syncing endpoints:", key)

	obj, endpExists, err := ingc.endpLister.GetByKey(key)
	if err != nil {
		ingc.syncQueue.requeue(task, err)
		return
	}

	if endpExists {
		ings := ingc.getIngForEndp(obj)

		for _, ing := range ings {
			if !ingc.isVarnishIngress(&ing) {
				continue
			}
			if !ingc.hasIngress(&ing) {
				continue
			}
			ingc.addOrUpdateIng(task, ing)
		}
	}
}

func (ingc *IngressController) sync(task Task) {
	log.Printf("Syncing %v", task.Key)

	switch task.Kind {
	case Ingress:
		ingc.syncIng(task)
	case Endpoints:
		ingc.syncEndp(task)
		return
	}
}

func (ingc *IngressController) syncIng(task Task) {
	key := task.Key
	ing, ingExists, err := ingc.ingLister.GetByKeySafe(key)
	if err != nil {
		ingc.syncQueue.requeue(task, err)
		return
	}

	if !ingExists {
		log.Print("Deleting Ingress:", key)

		err := ingc.vController.DeleteIngress(key)
		if err != nil {
			log.Printf("Error deleting configuration for %v: %v",
				key, err)
		}
		return
	}
	ingc.addOrUpdateIng(task, *ing)
}

func (ingc *IngressController) enqueueIngressForService(svc *api_v1.Service) {
	ings := ingc.getIngForSvc(svc)
	for _, ing := range ings {
		if !ingc.isVarnishIngress(&ing) {
			continue
		}
		if !ingc.hasIngress(&ing) {
			continue
		}
		ingc.syncQueue.enqueue(&ing)
	}
}

func (ingc *IngressController) getIngForSvc(svc *api_v1.Service) []extensions.Ingress {
	ings, err := ingc.ingLister.GetServiceIngress(svc)
	if err != nil {
		log.Printf("ignoring service %v: %v", svc.Name, err)
		return nil
	}
	return ings
}

func (ingc *IngressController) getIngForEndp(obj interface{}) []extensions.Ingress {
	var ings []extensions.Ingress
	endp := obj.(*api_v1.Endpoints)
	svcKey := endp.GetNamespace() + "/" + endp.GetName()
	svcObj, svcExists, err := ingc.svcLister.GetByKey(svcKey)
	if err != nil {
		log.Printf("error getting service %v from the cache: %v",
			svcKey, err)
	} else {
		if svcExists {
			ings = append(ings,
				ingc.getIngForSvc(svcObj.(*api_v1.Service))...)
		}
	}
	return ings
}

func (ingc *IngressController) ing2VCLSpec(ing *extensions.Ingress) (vcl.Spec, error) {
	vclSpec := vcl.Spec{}
	vclSpec.AllServices = make(map[string]vcl.Service)
	if ing.Spec.TLS != nil && len(ing.Spec.TLS) > 0 {
		log.Printf("TLS config currently ignored in Ingress %s",
			ing.ObjectMeta.Name)
	}
	if ing.Spec.Backend != nil {
		backend := ing.Spec.Backend
		addrs, err := ingc.ingBackend2Addrs(*backend, ing.Namespace)
		if err != nil {
			return vclSpec, err
		}
		vclSvc := vcl.Service{
			Name:      backend.ServiceName,
			Addresses: addrs,
		}
		vclSpec.DefaultService = vclSvc
		vclSpec.AllServices[backend.ServiceName] = vclSvc
	}
	for _, rule := range ing.Spec.Rules {
		if rule.Host == "" {
			return vclSpec, fmt.Errorf("Ingress rule contains " +
				"empty Host")
		}
		vclRule := vcl.Rule{Host: rule.Host}
		vclRule.PathMap = make(map[string]vcl.Service)
		if rule.IngressRuleValue.HTTP == nil {
			vclSpec.Rules = append(vclSpec.Rules, vclRule)
			continue
		}
		for _, path := range rule.IngressRuleValue.HTTP.Paths {
			addrs, err := ingc.ingBackend2Addrs(path.Backend,
				ing.Namespace)
			if err != nil {
				return vclSpec, err
			}
			vclSvc := vcl.Service{
				Name:      path.Backend.ServiceName,
				Addresses: addrs,
			}
			vclRule.PathMap[path.Path] = vclSvc
			vclSpec.AllServices[path.Backend.ServiceName] = vclSvc
		}
		vclSpec.Rules = append(vclSpec.Rules, vclRule)
	}
	return vclSpec, nil
}

func (ingc *IngressController) ingBackend2Addrs(backend extensions.IngressBackend,
	namespace string) ([]vcl.Address, error) {

	var addrs []vcl.Address
	svcKey := namespace + "/" + backend.ServiceName
	svcObj, ok, err := ingc.svcLister.GetByKey(svcKey)
	if err != nil {
		return addrs, err
	}
	if !ok {
		return addrs, fmt.Errorf("service %s does not exist", svcKey)
	}
	svc := svcObj.(*api_v1.Service)

	endps, err := ingc.endpLister.GetServiceEndpoints(svc)
	if err != nil {
		return addrs, fmt.Errorf("Error getting endpoints for service "+
			"%v: %v", svc, err)
	}

	targetPort := int32(0)
	ingSvcPort := backend.ServicePort
	for _, port := range svc.Spec.Ports {
		if (ingSvcPort.Type == intstr.Int &&
			port.Port == int32(ingSvcPort.IntValue())) ||
			(ingSvcPort.Type == intstr.String &&
				port.Name == ingSvcPort.String()) {

			targetPort, err = ingc.getTargetPort(&port, svc)
			if err != nil {
				return addrs, fmt.Errorf("Error determining "+
					"target port for port %v in Ingress: "+
					"%v", ingSvcPort, err)
			}
			break
		}
	}
	if targetPort == 0 {
		return addrs, fmt.Errorf("No port %v in service %s", ingSvcPort,
			svc.Name)
	}

	for _, subset := range endps.Subsets {
		for _, port := range subset.Ports {
			if port.Port == targetPort {
				for _, address := range subset.Addresses {
					addr := vcl.Address{
						IP:   address.IP,
						Port: port.Port,
					}
					addrs = append(addrs, addr)
				}
				return addrs, nil
			}
		}
	}
	return addrs, fmt.Errorf("No endpoints for target port %v in service "+
		"%s", targetPort, svc.Name)
}

func (ingc *IngressController) getTargetPort(svcPort *api_v1.ServicePort, svc *api_v1.Service) (int32, error) {
	if (svcPort.TargetPort == intstr.IntOrString{}) {
		return svcPort.Port, nil
	}

	if svcPort.TargetPort.Type == intstr.Int {
		return int32(svcPort.TargetPort.IntValue()), nil
	}

	pods, err := ingc.client.Core().Pods(svc.Namespace).
		List(meta_v1.ListOptions{
			LabelSelector: labels.Set(svc.Spec.Selector).String(),
		})
	if err != nil {
		return 0, fmt.Errorf("Error getting pod information: %v", err)
	}

	if len(pods.Items) == 0 {
		return 0, fmt.Errorf("No pods of service: %v", svc.Name)
	}

	pod := &pods.Items[0]

	portNum, err := FindPort(pod, svcPort)
	if err != nil {
		return 0, fmt.Errorf("Error finding named port %v in pod %s: "+
			"%v", svcPort, pod.Name, err)
	}

	return portNum, nil
}

// Check if resource ingress class annotation (if exists) matches
// ingress controller class
func (ingc *IngressController) isVarnishIngress(ing *extensions.Ingress) bool {
	if class, exists := ing.Annotations[ingressClassKey]; exists {
		return class == "varnish" || class == ""
	}
	return true
}

func (ingc *IngressController) hasIngress(ing *extensions.Ingress) bool {
	name := ing.ObjectMeta.Namespace + "/" + ing.ObjectMeta.Name
	return ingc.vController.HasIngress(name)
}

// isExternalServiceForStatus matches the service specified by the
// external-service arg
func (ingc *IngressController) isExternalServiceForStatus(svc *api_v1.Service) bool {
	// return ingc.statusUpdater.namespace == svc.Namespace &&
	// 	ingc.statusUpdater.externalServiceName == svc.Name
	return false
}
