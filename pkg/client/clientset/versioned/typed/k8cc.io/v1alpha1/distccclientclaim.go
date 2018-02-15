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

// DistccClientClaimsGetter has a method to return a DistccClientClaimInterface.
// A group's client should implement this interface.
type DistccClientClaimsGetter interface {
	DistccClientClaims(namespace string) DistccClientClaimInterface
}

// DistccClientClaimInterface has methods to work with DistccClientClaim resources.
type DistccClientClaimInterface interface {
	Create(*v1alpha1.DistccClientClaim) (*v1alpha1.DistccClientClaim, error)
	Update(*v1alpha1.DistccClientClaim) (*v1alpha1.DistccClientClaim, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.DistccClientClaim, error)
	List(opts v1.ListOptions) (*v1alpha1.DistccClientClaimList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClientClaim, err error)
	DistccClientClaimExpansion
}

// distccClientClaims implements DistccClientClaimInterface
type distccClientClaims struct {
	client rest.Interface
	ns     string
}

// newDistccClientClaims returns a DistccClientClaims
func newDistccClientClaims(c *K8ccV1alpha1Client, namespace string) *distccClientClaims {
	return &distccClientClaims{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the distccClientClaim, and returns the corresponding distccClientClaim object, and an error if there is any.
func (c *distccClientClaims) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClientClaim, err error) {
	result = &v1alpha1.DistccClientClaim{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclientclaims").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DistccClientClaims that match those selectors.
func (c *distccClientClaims) List(opts v1.ListOptions) (result *v1alpha1.DistccClientClaimList, err error) {
	result = &v1alpha1.DistccClientClaimList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclientclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested distccClientClaims.
func (c *distccClientClaims) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("distccclientclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a distccClientClaim and creates it.  Returns the server's representation of the distccClientClaim, and an error, if there is any.
func (c *distccClientClaims) Create(distccClientClaim *v1alpha1.DistccClientClaim) (result *v1alpha1.DistccClientClaim, err error) {
	result = &v1alpha1.DistccClientClaim{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("distccclientclaims").
		Body(distccClientClaim).
		Do().
		Into(result)
	return
}

// Update takes the representation of a distccClientClaim and updates it. Returns the server's representation of the distccClientClaim, and an error, if there is any.
func (c *distccClientClaims) Update(distccClientClaim *v1alpha1.DistccClientClaim) (result *v1alpha1.DistccClientClaim, err error) {
	result = &v1alpha1.DistccClientClaim{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("distccclientclaims").
		Name(distccClientClaim.Name).
		Body(distccClientClaim).
		Do().
		Into(result)
	return
}

// Delete takes name of the distccClientClaim and deletes it. Returns an error if one occurs.
func (c *distccClientClaims) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclientclaims").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *distccClientClaims) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclientclaims").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched distccClientClaim.
func (c *distccClientClaims) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClientClaim, err error) {
	result = &v1alpha1.DistccClientClaim{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("distccclientclaims").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
