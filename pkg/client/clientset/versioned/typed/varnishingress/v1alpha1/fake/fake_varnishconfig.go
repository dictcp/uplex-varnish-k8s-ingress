/*
 * Copyright (c) 2018 UPLEX Nils Goroll Systemoptimierung
 * All rights reserved
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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "code.uplex.de/uplex-varnish/k8s-ingress/pkg/apis/varnishingress/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVarnishConfigs implements VarnishConfigInterface
type FakeVarnishConfigs struct {
	Fake *FakeIngressV1alpha1
	ns   string
}

var varnishconfigsResource = schema.GroupVersionResource{Group: "ingress.varnish-cache.org", Version: "v1alpha1", Resource: "varnishconfigs"}

var varnishconfigsKind = schema.GroupVersionKind{Group: "ingress.varnish-cache.org", Version: "v1alpha1", Kind: "VarnishConfig"}

// Get takes name of the varnishConfig, and returns the corresponding varnishConfig object, and an error if there is any.
func (c *FakeVarnishConfigs) Get(name string, options v1.GetOptions) (result *v1alpha1.VarnishConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(varnishconfigsResource, c.ns, name), &v1alpha1.VarnishConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VarnishConfig), err
}

// List takes label and field selectors, and returns the list of VarnishConfigs that match those selectors.
func (c *FakeVarnishConfigs) List(opts v1.ListOptions) (result *v1alpha1.VarnishConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(varnishconfigsResource, varnishconfigsKind, c.ns, opts), &v1alpha1.VarnishConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.VarnishConfigList{}
	for _, item := range obj.(*v1alpha1.VarnishConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested varnishConfigs.
func (c *FakeVarnishConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(varnishconfigsResource, c.ns, opts))

}

// Create takes the representation of a varnishConfig and creates it.  Returns the server's representation of the varnishConfig, and an error, if there is any.
func (c *FakeVarnishConfigs) Create(varnishConfig *v1alpha1.VarnishConfig) (result *v1alpha1.VarnishConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(varnishconfigsResource, c.ns, varnishConfig), &v1alpha1.VarnishConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VarnishConfig), err
}

// Update takes the representation of a varnishConfig and updates it. Returns the server's representation of the varnishConfig, and an error, if there is any.
func (c *FakeVarnishConfigs) Update(varnishConfig *v1alpha1.VarnishConfig) (result *v1alpha1.VarnishConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(varnishconfigsResource, c.ns, varnishConfig), &v1alpha1.VarnishConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VarnishConfig), err
}

// Delete takes name of the varnishConfig and deletes it. Returns an error if one occurs.
func (c *FakeVarnishConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(varnishconfigsResource, c.ns, name), &v1alpha1.VarnishConfig{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVarnishConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(varnishconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.VarnishConfigList{})
	return err
}

// Patch applies the patch and returns the patched varnishConfig.
func (c *FakeVarnishConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.VarnishConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(varnishconfigsResource, c.ns, name, data, subresources...), &v1alpha1.VarnishConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VarnishConfig), err
}
