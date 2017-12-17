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

// This file was automatically generated by informer-gen

package v1alpha1

import (
	k8cc_io_v1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	versioned "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	internalinterfaces "github.com/mbrt/k8cc/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/mbrt/k8cc/pkg/client/listers/k8cc/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	time "time"
)

// DistccInformer provides access to a shared informer and lister for
// Distccs.
type DistccInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.DistccLister
}

type distccInformer struct {
	factory internalinterfaces.SharedInformerFactory
}

// NewDistccInformer constructs a new informer for Distcc type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDistccInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				return client.K8ccV1alpha1().Distccs(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				return client.K8ccV1alpha1().Distccs(namespace).Watch(options)
			},
		},
		&k8cc_io_v1alpha1.Distcc{},
		resyncPeriod,
		indexers,
	)
}

func defaultDistccInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewDistccInformer(client, v1.NamespaceAll, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
}

func (f *distccInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&k8cc_io_v1alpha1.Distcc{}, defaultDistccInformer)
}

func (f *distccInformer) Lister() v1alpha1.DistccLister {
	return v1alpha1.NewDistccLister(f.Informer().GetIndexer())
}