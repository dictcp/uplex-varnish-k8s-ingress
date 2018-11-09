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
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api_v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

// taskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type taskQueue struct {
	// queue is the work queue the worker polls
	queue *workqueue.Type
	// sync is called for each item in the queue
	sync func(Task)
	// workerDone is closed when the worker exits
	workerDone chan struct{}
}

func (t *taskQueue) run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

// enqueue enqueues ns/name of the given api object in the task queue.
func (t *taskQueue) enqueue(obj interface{}) {
	key, err := keyFunc(obj)
	if err != nil {
		log.Printf("Couldn't get key for object %v: %v", obj, err)
		return
	}

	task, err := NewTask(key, obj)
	if err != nil {
		log.Printf("Couldn't create a task for object %v: %v", obj, err)
		return
	}

	log.Print("Adding an element with a key:", task.Key)

	t.queue.Add(task)
}

func (t *taskQueue) requeue(task Task, err error) {
	log.Printf("Requeuing %v, err %v", task.Key, err)
	t.queue.Add(task)
}

func (t *taskQueue) requeueAfter(task Task, err error, after time.Duration) {
	log.Printf("Requeuing %v after %s, err %v", task.Key, after.String(),
		err)
	go func(task Task, after time.Duration) {
		time.Sleep(after)
		t.queue.Add(task)
	}(task, after)
}

// worker processes work in the queue through sync.
func (t *taskQueue) worker() {
	for {
		task, quit := t.queue.Get()
		if quit {
			close(t.workerDone)
			return
		}
		log.Printf("Syncing %v", task.(Task).Key)
		t.sync(task.(Task))
		t.queue.Done(task)
	}
}

// shutdown shuts down the work queue and waits for the worker to ACK
func (t *taskQueue) shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(syncFn func(Task)) *taskQueue {
	return &taskQueue{
		queue:      workqueue.New(),
		sync:       syncFn,
		workerDone: make(chan struct{}),
	}
}

// Kind represents the kind of the Kubernetes resources of a task
type Kind int

const (
	// Ingress resource
	Ingress = iota
	// Endpoints resource
	Endpoints
	// Service resource
	Service
)

// Task is an element of a taskQueue
type Task struct {
	Kind Kind
	Key  string
}

// NewTask creates a new task
func NewTask(key string, obj interface{}) (Task, error) {
	var k Kind
	switch t := obj.(type) {
	case *extensions.Ingress:
//		ing := obj.(*extensions.Ingress)
		k = Ingress
	case *api_v1.Endpoints:
		k = Endpoints
	case *api_v1.Service:
		k = Service
	default:
		return Task{}, fmt.Errorf("Unknown type: %v", t)
	}

	return Task{k, key}, nil
}

// compareLinks returns true if the 2 self links are equal.
// func compareLinks(l1, l2 string) bool {
// 	// TODO: These can be partial links
// 	return l1 == l2 && l1 != ""
// }

// StoreToIngressLister makes a Store that lists Ingress.
// TODO: Move this to cache/listers post 1.1.
type StoreToIngressLister struct {
	cache.Store
}

// GetByKeySafe calls Store.GetByKeySafe and returns a copy of the ingress so it is
// safe to modify.
func (s *StoreToIngressLister) GetByKeySafe(key string) (ing *extensions.Ingress, exists bool, err error) {
	item, exists, err := s.Store.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	ing = item.(*extensions.Ingress).DeepCopy()
	return
}

// List lists all Ingress' in the store.
func (s *StoreToIngressLister) List() (ing extensions.IngressList, err error) {
	for _, m := range s.Store.List() {
		ing.Items = append(ing.Items, *(m.(*extensions.Ingress)).DeepCopy())
	}
	return ing, nil
}

// GetServiceIngress gets all the Ingress' that have rules pointing to a service.
// Note that this ignores services without the right nodePorts.
func (s *StoreToIngressLister) GetServiceIngress(svc *api_v1.Service) (ings []extensions.Ingress, err error) {
	for _, m := range s.Store.List() {
		ing := *m.(*extensions.Ingress).DeepCopy()
		if ing.Namespace != svc.Namespace {
			continue
		}
		if ing.Spec.Backend != nil {
			if ing.Spec.Backend.ServiceName == svc.Name {
				ings = append(ings, ing)
			}
		}
		for _, rules := range ing.Spec.Rules {
			if rules.IngressRuleValue.HTTP == nil {
				continue
			}
			for _, p := range rules.IngressRuleValue.HTTP.Paths {
				if p.Backend.ServiceName == svc.Name {
					ings = append(ings, ing)
				}
			}
		}
	}
	if len(ings) == 0 {
		err = fmt.Errorf("No ingress for service %v", svc.Name)
	}
	return
}

// StoreToEndpointLister makes a Store that lists Endpoints
type StoreToEndpointLister struct {
	cache.Store
}

// GetServiceEndpoints returns the endpoints of a service, matched on service name.
func (s *StoreToEndpointLister) GetServiceEndpoints(svc *api_v1.Service) (ep api_v1.Endpoints, err error) {
	for _, m := range s.Store.List() {
		ep = *m.(*api_v1.Endpoints)
		if svc.Name == ep.Name && svc.Namespace == ep.Namespace {
			return ep, nil
		}
	}
	err = fmt.Errorf("could not find endpoints for service: %v", svc.Name)
	return
}

// FindPort locates the container port for the given pod and portName.  If the
// targetPort is a number, use that.  If the targetPort is a string, look that
// string up in all named ports in all containers in the target pod.  If no
// match is found, fail.
func FindPort(pod *api_v1.Pod, svcPort *api_v1.ServicePort) (int32, error) {
	portName := svcPort.TargetPort
	switch portName.Type {
	case intstr.String:
		name := portName.StrVal
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == name &&
					port.Protocol == svcPort.Protocol {
					return port.ContainerPort, nil
				}
			}
		}
	case intstr.Int:
		return int32(portName.IntValue()), nil
	}

	return 0, fmt.Errorf("no suitable port for manifest: %s", pod.UID)
}
