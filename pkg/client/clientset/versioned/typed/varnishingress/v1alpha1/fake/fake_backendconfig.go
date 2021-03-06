/*
 * Copyright (c) 2019 UPLEX Nils Goroll Systemoptimierung
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

// FakeBackendConfigs implements BackendConfigInterface
type FakeBackendConfigs struct {
	Fake *FakeIngressV1alpha1
	ns   string
}

var backendconfigsResource = schema.GroupVersionResource{Group: "ingress.varnish-cache.org", Version: "v1alpha1", Resource: "backendconfigs"}

var backendconfigsKind = schema.GroupVersionKind{Group: "ingress.varnish-cache.org", Version: "v1alpha1", Kind: "BackendConfig"}

// Get takes name of the backendConfig, and returns the corresponding backendConfig object, and an error if there is any.
func (c *FakeBackendConfigs) Get(name string, options v1.GetOptions) (result *v1alpha1.BackendConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(backendconfigsResource, c.ns, name), &v1alpha1.BackendConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.BackendConfig), err
}

// List takes label and field selectors, and returns the list of BackendConfigs that match those selectors.
func (c *FakeBackendConfigs) List(opts v1.ListOptions) (result *v1alpha1.BackendConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(backendconfigsResource, backendconfigsKind, c.ns, opts), &v1alpha1.BackendConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.BackendConfigList{ListMeta: obj.(*v1alpha1.BackendConfigList).ListMeta}
	for _, item := range obj.(*v1alpha1.BackendConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested backendConfigs.
func (c *FakeBackendConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(backendconfigsResource, c.ns, opts))

}

// Create takes the representation of a backendConfig and creates it.  Returns the server's representation of the backendConfig, and an error, if there is any.
func (c *FakeBackendConfigs) Create(backendConfig *v1alpha1.BackendConfig) (result *v1alpha1.BackendConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(backendconfigsResource, c.ns, backendConfig), &v1alpha1.BackendConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.BackendConfig), err
}

// Update takes the representation of a backendConfig and updates it. Returns the server's representation of the backendConfig, and an error, if there is any.
func (c *FakeBackendConfigs) Update(backendConfig *v1alpha1.BackendConfig) (result *v1alpha1.BackendConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(backendconfigsResource, c.ns, backendConfig), &v1alpha1.BackendConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.BackendConfig), err
}

// Delete takes name of the backendConfig and deletes it. Returns an error if one occurs.
func (c *FakeBackendConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(backendconfigsResource, c.ns, name), &v1alpha1.BackendConfig{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBackendConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(backendconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.BackendConfigList{})
	return err
}

// Patch applies the patch and returns the patched backendConfig.
func (c *FakeBackendConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.BackendConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(backendconfigsResource, c.ns, name, pt, data, subresources...), &v1alpha1.BackendConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.BackendConfig), err
}
