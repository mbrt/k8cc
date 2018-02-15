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

package v1alpha1

import (
	v1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	scheme "github.com/mbrt/k8cc/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// DistccClaimsGetter has a method to return a DistccClaimInterface.
// A group's client should implement this interface.
type DistccClaimsGetter interface {
	DistccClaims(namespace string) DistccClaimInterface
}

// DistccClaimInterface has methods to work with DistccClaim resources.
type DistccClaimInterface interface {
	Create(*v1alpha1.DistccClaim) (*v1alpha1.DistccClaim, error)
	Update(*v1alpha1.DistccClaim) (*v1alpha1.DistccClaim, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.DistccClaim, error)
	List(opts v1.ListOptions) (*v1alpha1.DistccClaimList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClaim, err error)
	DistccClaimExpansion
}

// distccClaims implements DistccClaimInterface
type distccClaims struct {
	client rest.Interface
	ns     string
}

// newDistccClaims returns a DistccClaims
func newDistccClaims(c *K8ccV1alpha1Client, namespace string) *distccClaims {
	return &distccClaims{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the distccClaim, and returns the corresponding distccClaim object, and an error if there is any.
func (c *distccClaims) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClaim, err error) {
	result = &v1alpha1.DistccClaim{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclaims").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DistccClaims that match those selectors.
func (c *distccClaims) List(opts v1.ListOptions) (result *v1alpha1.DistccClaimList, err error) {
	result = &v1alpha1.DistccClaimList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested distccClaims.
func (c *distccClaims) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("distccclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a distccClaim and creates it.  Returns the server's representation of the distccClaim, and an error, if there is any.
func (c *distccClaims) Create(distccClaim *v1alpha1.DistccClaim) (result *v1alpha1.DistccClaim, err error) {
	result = &v1alpha1.DistccClaim{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("distccclaims").
		Body(distccClaim).
		Do().
		Into(result)
	return
}

// Update takes the representation of a distccClaim and updates it. Returns the server's representation of the distccClaim, and an error, if there is any.
func (c *distccClaims) Update(distccClaim *v1alpha1.DistccClaim) (result *v1alpha1.DistccClaim, err error) {
	result = &v1alpha1.DistccClaim{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("distccclaims").
		Name(distccClaim.Name).
		Body(distccClaim).
		Do().
		Into(result)
	return
}

// Delete takes name of the distccClaim and deletes it. Returns an error if one occurs.
func (c *distccClaims) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclaims").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *distccClaims) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclaims").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched distccClaim.
func (c *distccClaims) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClaim, err error) {
	result = &v1alpha1.DistccClaim{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("distccclaims").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
