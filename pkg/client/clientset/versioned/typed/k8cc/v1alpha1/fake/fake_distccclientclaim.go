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

// FakeDistccClientClaims implements DistccClientClaimInterface
type FakeDistccClientClaims struct {
	Fake *FakeK8ccV1alpha1
	ns   string
}

var distccclientclaimsResource = schema.GroupVersionResource{Group: "k8cc.io", Version: "v1alpha1", Resource: "distccclientclaims"}

var distccclientclaimsKind = schema.GroupVersionKind{Group: "k8cc.io", Version: "v1alpha1", Kind: "DistccClientClaim"}

// Get takes name of the distccClientClaim, and returns the corresponding distccClientClaim object, and an error if there is any.
func (c *FakeDistccClientClaims) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClientClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(distccclientclaimsResource, c.ns, name), &v1alpha1.DistccClientClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClientClaim), err
}

// List takes label and field selectors, and returns the list of DistccClientClaims that match those selectors.
func (c *FakeDistccClientClaims) List(opts v1.ListOptions) (result *v1alpha1.DistccClientClaimList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(distccclientclaimsResource, distccclientclaimsKind, c.ns, opts), &v1alpha1.DistccClientClaimList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DistccClientClaimList{}
	for _, item := range obj.(*v1alpha1.DistccClientClaimList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested distccClientClaims.
func (c *FakeDistccClientClaims) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(distccclientclaimsResource, c.ns, opts))

}

// Create takes the representation of a distccClientClaim and creates it.  Returns the server's representation of the distccClientClaim, and an error, if there is any.
func (c *FakeDistccClientClaims) Create(distccClientClaim *v1alpha1.DistccClientClaim) (result *v1alpha1.DistccClientClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(distccclientclaimsResource, c.ns, distccClientClaim), &v1alpha1.DistccClientClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClientClaim), err
}

// Update takes the representation of a distccClientClaim and updates it. Returns the server's representation of the distccClientClaim, and an error, if there is any.
func (c *FakeDistccClientClaims) Update(distccClientClaim *v1alpha1.DistccClientClaim) (result *v1alpha1.DistccClientClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(distccclientclaimsResource, c.ns, distccClientClaim), &v1alpha1.DistccClientClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClientClaim), err
}

// Delete takes name of the distccClientClaim and deletes it. Returns an error if one occurs.
func (c *FakeDistccClientClaims) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(distccclientclaimsResource, c.ns, name), &v1alpha1.DistccClientClaim{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDistccClientClaims) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(distccclientclaimsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.DistccClientClaimList{})
	return err
}

// Patch applies the patch and returns the patched distccClientClaim.
func (c *FakeDistccClientClaims) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClientClaim, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(distccclientclaimsResource, c.ns, name, data, subresources...), &v1alpha1.DistccClientClaim{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DistccClientClaim), err
}
