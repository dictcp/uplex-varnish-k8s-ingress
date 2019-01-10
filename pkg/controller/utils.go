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

	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	vcr_v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	"code.uplex.de/uplex-varnish/k8s-ingress/pkg/varnish/vcl"
)

const (
	labelKey = "app"
	labelVal = "varnish-ingress"
)

var (
	varnishIngressSet = labels.Set(map[string]string{
		labelKey: labelVal,
	})
	// Selector for use in List() operations to find resources
	// with the app:varnish-ingress label.
	varnishIngressSelector = labels.SelectorFromSet(varnishIngressSet)
)

// getServiceEndpoints returns the endpoints of a service, matched on
// service name.
func (worker *NamespaceWorker) getServiceEndpoints(
	svc *api_v1.Service) (ep *api_v1.Endpoints, err error) {

	eps, err := worker.endp.List(labels.Everything())
	if err != nil {
		return
	}
	for _, ep := range eps {
		if svc.Name == ep.Name && svc.Namespace == ep.Namespace {
			return ep, nil
		}
	}
	err = fmt.Errorf("could not find endpoints for service: %s/%s",
		svc.Namespace, svc.Name)
	return
}

// endpsTargetPort2Addrs returns a list of addresses for VCL backend
// config, given the Endpoints of a Service and a target port number
// for their Pods.
func endpsTargetPort2Addrs(
	svc *api_v1.Service,
	endps *api_v1.Endpoints,
	targetPort int32) ([]vcl.Address, error) {

	var addrs []vcl.Address
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
	return addrs, fmt.Errorf("No endpoints for service port %d in service "+
		"%s/%s", targetPort, svc.Namespace, svc.Name)
}

// findPort returns the container port number for a Pod and
// ServicePort. If the targetPort is a string, search for a matching
// named ports in the specs for all containers in the Pod.
func findPort(pod *api_v1.Pod, svcPort *api_v1.ServicePort) (int32, error) {
	portName := svcPort.TargetPort
	switch portName.Type {
	case intstr.Int:
		return int32(portName.IntValue()), nil
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
	}

	return 0, fmt.Errorf("No port number found for ServicePort %s and Pod "+
		"%s/%s", svcPort.Name, pod.Namespace, pod.Name)
}

// getPodsForSvc queries the API for the Pods in a Service.
func (worker *NamespaceWorker) getPods(
	svc *api_v1.Service) (*api_v1.PodList, error) {

	return worker.client.CoreV1().Pods(svc.Namespace).
		List(meta_v1.ListOptions{
			LabelSelector: labels.Set(svc.Spec.Selector).String(),
		})
}

// getTargetPort returns a target port number for the Pods of a Service,
// given a ServicePort.
func (worker *NamespaceWorker) getTargetPort(svcPort *api_v1.ServicePort,
	svc *api_v1.Service) (int32, error) {

	if (svcPort.TargetPort == intstr.IntOrString{}) {
		return svcPort.Port, nil
	}

	if svcPort.TargetPort.Type == intstr.Int {
		return int32(svcPort.TargetPort.IntValue()), nil
	}

	pods, err := worker.getPods(svc)
	if err != nil {
		return 0, fmt.Errorf("Error getting pod information: %v", err)
	}
	if len(pods.Items) == 0 {
		return 0, fmt.Errorf("No pods of service: %v", svc.Name)
	}

	pod := &pods.Items[0]
	portNum, err := findPort(pod, svcPort)
	if err != nil {
		return 0, fmt.Errorf("Error finding named port %s in pod %s/%s"+
			": %v", svcPort.Name, pod.Namespace, pod.Name, err)
	}

	return portNum, nil
}

// XXX a validation webhook should do this.
// Assume that validation for the CustomResource has already checked
// the Timeout, Interval and Initial fields, and that Window and
// Threshold have been checked for permitted ranges.
func validateProbe(probe *vcr_v1alpha1.ProbeSpec) error {
	if probe == nil {
		return nil
	}
	if probe.Window != nil && probe.Threshold != nil &&
		*probe.Threshold > *probe.Window {
		return fmt.Errorf("Probe Threshold (%d) may not be greater "+
			"than Window (%d)", probe.Threshold, probe.Window)
	}
	return nil
}
