/*
Copyright 2017 The Kubernetes Authors.

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

// FakeDistccs implements DistccInterface
type FakeDistccs struct {
	Fake *FakeK8ccV1alpha1
	ns   string
}

var distccsResource = schema.GroupVersionResource{Group: "k8cc.io", Version: "v1alpha1", Resource: "distccs"}

var distccsKind = schema.GroupVersionKind{Group: "k8cc.io", Version: "v1alpha1", Kind: "Distcc"}

// Get takes name of the distcc, and returns the corresponding distcc object, and an error if there is any.
func (c *FakeDistccs) Get(name string, options v1.GetOptions) (result *v1alpha1.Distcc, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(distccsResource, c.ns, name), &v1alpha1.Distcc{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Distcc), err
}

// List takes label and field selectors, and returns the list of Distccs that match those selectors.
func (c *FakeDistccs) List(opts v1.ListOptions) (result *v1alpha1.DistccList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(distccsResource, distccsKind, c.ns, opts), &v1alpha1.DistccList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DistccList{}
	for _, item := range obj.(*v1alpha1.DistccList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested distccs.
func (c *FakeDistccs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(distccsResource, c.ns, opts))

}

// Create takes the representation of a distcc and creates it.  Returns the server's representation of the distcc, and an error, if there is any.
func (c *FakeDistccs) Create(distcc *v1alpha1.Distcc) (result *v1alpha1.Distcc, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(distccsResource, c.ns, distcc), &v1alpha1.Distcc{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Distcc), err
}

// Update takes the representation of a distcc and updates it. Returns the server's representation of the distcc, and an error, if there is any.
func (c *FakeDistccs) Update(distcc *v1alpha1.Distcc) (result *v1alpha1.Distcc, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(distccsResource, c.ns, distcc), &v1alpha1.Distcc{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Distcc), err
}

// Delete takes name of the distcc and deletes it. Returns an error if one occurs.
func (c *FakeDistccs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(distccsResource, c.ns, name), &v1alpha1.Distcc{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDistccs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(distccsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.DistccList{})
	return err
}

// Patch applies the patch and returns the patched distcc.
func (c *FakeDistccs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Distcc, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(distccsResource, c.ns, name, data, subresources...), &v1alpha1.Distcc{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Distcc), err
}
