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

// FakeDistccClaims implements DistccClaimInterface
type FakeDistccClaims struct {
	Fake *FakeK8ccV1alpha1
	ns   string
}

var distccclaimsResource = schema.GroupVersionResource{Group: "k8cc.io", Version: "v1alpha1", Resource: "distccclaims"}

var distccclaimsKind = schema.GroupVersionKind{Group: "k8cc.io", Version: "v1alpha1", Kind: "DistccClaim"}

// Get takes name of the distccClaim, and returns the corresponding distccClaim object, and an error if there is any.
func (c *FakeDistccClaims) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(distccclaimsResource, c.ns, name), &v1alpha1.DistccClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClaim), err
}

// List takes label and field selectors, and returns the list of DistccClaims that match those selectors.
func (c *FakeDistccClaims) List(opts v1.ListOptions) (result *v1alpha1.DistccClaimList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(distccclaimsResource, distccclaimsKind, c.ns, opts), &v1alpha1.DistccClaimList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DistccClaimList{}
	for _, item := range obj.(*v1alpha1.DistccClaimList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested distccClaims.
func (c *FakeDistccClaims) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(distccclaimsResource, c.ns, opts))

}

// Create takes the representation of a distccClaim and creates it.  Returns the server's representation of the distccClaim, and an error, if there is any.
func (c *FakeDistccClaims) Create(distccClaim *v1alpha1.DistccClaim) (result *v1alpha1.DistccClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(distccclaimsResource, c.ns, distccClaim), &v1alpha1.DistccClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClaim), err
}

// Update takes the representation of a distccClaim and updates it. Returns the server's representation of the distccClaim, and an error, if there is any.
func (c *FakeDistccClaims) Update(distccClaim *v1alpha1.DistccClaim) (result *v1alpha1.DistccClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(distccclaimsResource, c.ns, distccClaim), &v1alpha1.DistccClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClaim), err
}

// Delete takes name of the distccClaim and deletes it. Returns an error if one occurs.
func (c *FakeDistccClaims) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(distccclaimsResource, c.ns, name), &v1alpha1.DistccClaim{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDistccClaims) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(distccclaimsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.DistccClaimList{})
	return err
}

// Patch applies the patch and returns the patched distccClaim.
func (c *FakeDistccClaims) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(distccclaimsResource, c.ns, name, data, subresources...), &v1alpha1.DistccClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClaim), err
}
