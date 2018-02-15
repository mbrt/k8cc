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

// DistccClientsGetter has a method to return a DistccClientInterface.
// A group's client should implement this interface.
type DistccClientsGetter interface {
	DistccClients(namespace string) DistccClientInterface
}

// DistccClientInterface has methods to work with DistccClient resources.
type DistccClientInterface interface {
	Create(*v1alpha1.DistccClient) (*v1alpha1.DistccClient, error)
	Update(*v1alpha1.DistccClient) (*v1alpha1.DistccClient, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.DistccClient, error)
	List(opts v1.ListOptions) (*v1alpha1.DistccClientList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClient, err error)
	DistccClientExpansion
}

// distccClients implements DistccClientInterface
type distccClients struct {
	client rest.Interface
	ns     string
}

// newDistccClients returns a DistccClients
func newDistccClients(c *K8ccV1alpha1Client, namespace string) *distccClients {
	return &distccClients{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the distccClient, and returns the corresponding distccClient object, and an error if there is any.
func (c *distccClients) Get(name string, options v1.GetOptions) (result *v1alpha1.DistccClient, err error) {
	result = &v1alpha1.DistccClient{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclients").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DistccClients that match those selectors.
func (c *distccClients) List(opts v1.ListOptions) (result *v1alpha1.DistccClientList, err error) {
	result = &v1alpha1.DistccClientList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("distccclients").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested distccClients.
func (c *distccClients) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("distccclients").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a distccClient and creates it.  Returns the server's representation of the distccClient, and an error, if there is any.
func (c *distccClients) Create(distccClient *v1alpha1.DistccClient) (result *v1alpha1.DistccClient, err error) {
	result = &v1alpha1.DistccClient{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("distccclients").
		Body(distccClient).
		Do().
		Into(result)
	return
}

// Update takes the representation of a distccClient and updates it. Returns the server's representation of the distccClient, and an error, if there is any.
func (c *distccClients) Update(distccClient *v1alpha1.DistccClient) (result *v1alpha1.DistccClient, err error) {
	result = &v1alpha1.DistccClient{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("distccclients").
		Name(distccClient.Name).
		Body(distccClient).
		Do().
		Into(result)
	return
}

// Delete takes name of the distccClient and deletes it. Returns an error if one occurs.
func (c *distccClients) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclients").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *distccClients) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("distccclients").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched distccClient.
func (c *distccClients) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DistccClient, err error) {
	result = &v1alpha1.DistccClient{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("distccclients").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
