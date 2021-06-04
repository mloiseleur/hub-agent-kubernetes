/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	traefikv1alpha1 "github.com/traefik/hub-agent/pkg/crd/api/traefik/v1alpha1"
	versioned "github.com/traefik/hub-agent/pkg/crd/generated/client/traefik/clientset/versioned"
	internalinterfaces "github.com/traefik/hub-agent/pkg/crd/generated/client/traefik/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/traefik/hub-agent/pkg/crd/generated/client/traefik/listers/traefik/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// TraefikServiceInformer provides access to a shared informer and lister for
// TraefikServices.
type TraefikServiceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.TraefikServiceLister
}

type traefikServiceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewTraefikServiceInformer constructs a new informer for TraefikService type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewTraefikServiceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredTraefikServiceInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredTraefikServiceInformer constructs a new informer for TraefikService type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredTraefikServiceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TraefikV1alpha1().TraefikServices(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.TraefikV1alpha1().TraefikServices(namespace).Watch(context.TODO(), options)
			},
		},
		&traefikv1alpha1.TraefikService{},
		resyncPeriod,
		indexers,
	)
}

func (f *traefikServiceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredTraefikServiceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *traefikServiceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&traefikv1alpha1.TraefikService{}, f.defaultInformer)
}

func (f *traefikServiceInformer) Lister() v1alpha1.TraefikServiceLister {
	return v1alpha1.NewTraefikServiceLister(f.Informer().GetIndexer())
}
