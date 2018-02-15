/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fake

import (
	v1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeDistccClients implements DistccClientInterface
type FakeDistccClients struct {
	Fake *FakeK8ccV1alpha1
	ns   string
}

var distccclientsResource = schema.GroupVersionResource{Group: "k8cc.io", Version: "v1alpha1", Resource: "distccclients"}

var distccclientsKind = schema.GroupVersionKind{Group: "k8cc.io", Version: "v1alpha1", Kind: "DistccClient"}

// Get takes name of the distccClient, and returns the corresponding distccClient object, and an error if there is any.
func (c *FakeDistccClients) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(distccclientsResource, c.ns, name), &v1alpha1.DistccClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClient), err
}

// List takes label and field selectors, and returns the list of DistccClients that match those selectors.
func (c *FakeDistccClients) List(opts v1.ListOptions) (result *v1alpha1.DistccClientList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(distccclientsResource, distccclientsKind, c.ns, opts), &v1alpha1.DistccClientList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DistccClientList{}
	for _, item := range obj.(*v1alpha1.DistccClientList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested distccClients.
func (c *FakeDistccClients) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(distccclientsResource, c.ns, opts))

}

// Create takes the representation of a distccClient and creates it.  Returns the server's representation of the distccClient, and an error, if there is any.
func (c *FakeDistccClients) Create(distccClient *v1alpha1.DistccClient) (result *v1alpha1.DistccClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(distccclientsResource, c.ns, distccClient), &v1alpha1.DistccClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClient), err
}

// Update takes the representation of a distccClient and updates it. Returns the server's representation of the distccClient, and an error, if there is any.
func (c *FakeDistccClients) Update(distccClient *v1alpha1.DistccClient) (result *v1alpha1.DistccClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(distccclientsResource, c.ns, distccClient), &v1alpha1.DistccClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClient), err
}

// Delete takes name of the distccClient and deletes it. Returns an error if one occurs.
func (c *FakeDistccClients) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(distccclientsResource, c.ns, name), &v1alpha1.DistccClient{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDistccClients) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(distccclientsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.DistccClientList{})
	return err
}

// Patch applies the patch and returns the patched distccClient.
func (c *FakeDistccClients) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(distccclientsResource, c.ns, name, data, subresources...), &v1alpha1.DistccClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClient), err
}
