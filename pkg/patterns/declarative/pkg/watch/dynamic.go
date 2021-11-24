/*
Copyright 2019 The Kubernetes Authors.

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

package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WatchDelay is the time between a Watch being dropped and attempting to resume it
const WatchDelay = 30 * time.Second

// NewDynamicWatch constructs a watcher for unstructured objects.
// Deprecated: avoid using directly; will move to internal in future.
func NewDynamicWatch(restMapper meta.RESTMapper, client dynamic.Interface) (*dynamicWatch, chan event.GenericEvent, error) {
	dw := &dynamicWatch{
		events:     make(chan event.GenericEvent),
		restMapper: restMapper,
		client:     client,
	}

	return dw, dw.events, nil
}

type dynamicWatch struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper
	events     chan event.GenericEvent

	// lastRV caches the last reported resource version.
	// This helps us avoid sending duplicate events (e.g. on a rewatch)
	lastRV sync.Map
}

func (dw *dynamicWatch) newDynamicClient(gvk schema.GroupVersionKind, namespace string) (dynamic.ResourceInterface, error) {
	mapping, err := dw.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	resource := dw.client.Resource(mapping.Resource)
	if namespace == "" {
		return resource, nil
	} else {
		return resource.Namespace(namespace), nil
	}
}

// Add registers a watch for changes to 'trigger' filtered by 'options' to raise an event on 'target'
func (dw *dynamicWatch) Add(trigger schema.GroupVersionKind, options metav1.ListOptions, filterNamespace string, target metav1.ObjectMeta) error {
	client, err := dw.newDynamicClient(trigger, filterNamespace)
	if err != nil {
		return fmt.Errorf("creating client for (%s): %v", trigger.String(), err)
	}

	dw.lastRV = sync.Map{}

	go func() {
		for {
			dw.watchUntilClosed(client, trigger, options, filterNamespace, target)

			time.Sleep(WatchDelay)
		}
	}()

	return nil
}

var _ client.Object = clientObject{}

// clientObject is a concrete client.Object to pass to watch events.
type clientObject struct {
	runtime.Object
	*metav1.ObjectMeta
}

// A Watch will be closed when the pod loses connection to the API server.
// If a Watch is opened with no ResourceVersion then we will recieve an 'ADDED'
// event for all Watch objects[1]. This will result in 'overnotification'
// from this Watch but it will ensure we always Reconcile when needed`.
//
// [1] https://github.com/kubernetes/kubernetes/issues/54878#issuecomment-357575276
func (dw *dynamicWatch) watchUntilClosed(client dynamic.ResourceInterface, trigger schema.GroupVersionKind, options metav1.ListOptions, filterNamespace string, target metav1.ObjectMeta) {
	log := log.Log

	// Though we don't use the resource version, we allow bookmarks to help keep TCP connections healthy.
	options.AllowWatchBookmarks = true

	events, err := client.Watch(context.TODO(), options)
	if err != nil {
		log.WithValues("kind", trigger.String()).WithValues("namespace", filterNamespace).WithValues("labels", options.LabelSelector).Error(err, "failed to add watch to dynamic client")
		return
	}

	log.WithValues("kind", trigger.String()).WithValues("namespace", filterNamespace).WithValues("labels", options.LabelSelector).Info("watch began")

	// Always clean up watchers
	defer events.Stop()

	for clientEvent := range events.ResultChan() {
		switch clientEvent.Type {
		case watch.Bookmark:
			// not an object change, we ignore it
			continue
		case watch.Error:
			log.Error(fmt.Errorf("unexpected error from watch: %v", clientEvent.Object), "error during watch")
			return
		}

		u := clientEvent.Object.(*unstructured.Unstructured)
		key := types.NamespacedName{Namespace: u.GetNamespace(), Name: u.GetName()}
		rv := u.GetResourceVersion()

		switch clientEvent.Type {
		case watch.Deleted:
			// stop lastRV growing indefinitely
			dw.lastRV.Delete(key)
			// We always send the delete notification
		case watch.Added, watch.Modified:
			if previousRV, found := dw.lastRV.Load(key); found && previousRV == rv {
				// Don't send spurious invalidations
				continue
			}
			dw.lastRV.Store(key, rv)
		}

		log.WithValues("type", clientEvent.Type).WithValues("kind", trigger.String()).WithValues("name", key.Name, "namespace", key.Namespace).Info("broadcasting event")
		dw.events <- event.GenericEvent{Object: clientObject{Object: clientEvent.Object, ObjectMeta: &target}}
	}

	log.WithValues("kind", trigger.String()).WithValues("namespace", filterNamespace).WithValues("labels", options.LabelSelector).Info("watch closed")

	return
}
