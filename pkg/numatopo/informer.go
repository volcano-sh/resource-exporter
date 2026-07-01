/*
Copyright 2026 The Volcano Authors.

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

package numatopo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"volcano.sh/apis/pkg/apis/nodeinfo/v1alpha1"
	"volcano.sh/apis/pkg/client/clientset/versioned"
	informers "volcano.sh/apis/pkg/client/informers/externalversions"
	listers "volcano.sh/apis/pkg/client/listers/nodeinfo/v1alpha1"
)

// NumatopoCache manages the informer and lister for Numatopology resources.
// It provides a local cache to avoid frequent API server calls.
type NumatopoCache struct {
	factory  informers.SharedInformerFactory
	lister   listers.NumatopologyLister
	informer cache.SharedIndexInformer
	nodeName string
}

// NewNumatopoCache creates a new NumatopoCache with filtered informer.
// The informer only watches the Numatopology resource for the specified node.
func NewNumatopoCache(client versioned.Interface, nodeName string) *NumatopoCache {
	// Use FieldSelector to filter, only watch the current node's resource
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
		}),
	)

	numaInformer := factory.Nodeinfo().V1alpha1().Numatopologies()

	c := &NumatopoCache{
		factory:  factory,
		lister:   numaInformer.Lister(),
		informer: numaInformer.Informer(),
		nodeName: nodeName,
	}

	return c
}

// Start starts the informer goroutine.
// This method is non-blocking and starts the list-watch mechanism in background.
func (c *NumatopoCache) Start(stopCh <-chan struct{}) {
	klog.V(2).Infof("Starting Numatopology informer for node %s", c.nodeName)
	c.factory.Start(stopCh)
}

// WaitForCacheSync waits for the informer cache to be synced.
// Returns true if the cache was synced successfully, false otherwise.
func (c *NumatopoCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	klog.V(2).Infof("Waiting for Numatopology informer cache to sync")
	return cache.WaitForCacheSync(stopCh, c.informer.HasSynced)
}

// Get retrieves the Numatopology resource for the current node from local cache.
// This method does not make any API calls - it reads from the local cache.
// Returns nil if the resource does not exist in the cache.
func (c *NumatopoCache) Get() (*v1alpha1.Numatopology, error) {
	return c.lister.Get(c.nodeName)
}

// HasSynced returns true if the informer cache has been synced at least once.
func (c *NumatopoCache) HasSynced() bool {
	return c.informer.HasSynced()
}
