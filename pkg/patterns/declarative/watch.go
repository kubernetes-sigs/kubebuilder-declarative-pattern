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

package declarative

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/watch"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target"
)

// hookableReconciler is implemented by a reconciler that we can hook
type hookableReconciler interface {
	AddHook(hook Hook)
}

type DynamicWatch interface {
	// Add registers a watch for changes to 'trigger' filtered by 'options' to raise an event on 'target'.
	// If namespace is specified, the watch will be restricted to that namespace.
	Add(trigger schema.GroupVersionKind, options metav1.ListOptions, namespace string, target metav1.ObjectMeta) error
}

// WatchChildrenOptions configures how we want to watch children.
type WatchChildrenOptions struct {
	// Manager is used as a factory for the default RESTConfig and the RESTMapper.
	Manager ctrl.Manager

	// RESTConfig is the configuration for connecting to the cluster.
	RESTConfig *rest.Config

	// LabelMaker is used to build the labels we should watch on.
	LabelMaker LabelMaker

	// Controller contains the controller itself
	Controller controller.Controller

	// Reconciler lets us hook into the post-apply lifecycle event.
	Reconciler hookableReconciler

	// ScopeWatchesToNamespace controls whether watches are per-namespace.
	// This allows for more narrowly scoped RBAC permissions, at the cost of more watches.
	ScopeWatchesToNamespace bool
}

// WatchAll creates a Watch on ctrl for all objects reconciled by recnl.
// Deprecated: prefer WatchChildren (and consider setting ScopeWatchesToNamespace)
func WatchAll(config *rest.Config, ctrl controller.Controller, reconciler hookableReconciler, labelMaker LabelMaker) (chan struct{}, error) {
	options := WatchChildrenOptions{
		RESTConfig:              config,
		Controller:              ctrl,
		Reconciler:              reconciler,
		LabelMaker:              labelMaker,
		ScopeWatchesToNamespace: false,
	}
	return WatchChildren(options)
}

// WatchChildren sets up watching of the objects applied by a controller.
func WatchChildren(options WatchChildrenOptions) (chan struct{}, error) {
	// Inject a stop channel that will never close. The controller does not have a concept of
	// shutdown, so there is no opportunity to stop the watch.
	stopChannel := make(chan struct{})

	if options.LabelMaker == nil {
		return nil, fmt.Errorf("labelMaker is required to scope watches")
	}

	restConfig := options.RESTConfig
	if restConfig == nil {
		if options.Manager != nil {
			restConfig = options.Manager.GetConfig()
		} else {
			return nil, fmt.Errorf("RESTConfig or Manager should be set")
		}
	}

	var restMapper meta.RESTMapper
	if options.Manager != nil {
		restMapper = options.Manager.GetRESTMapper()
	} else {
		rm, err := apiutil.NewDiscoveryRESTMapper(restConfig)
		if err != nil {
			return nil, err
		}
		restMapper = rm
	}

	afterApplyHook := &afterApplyHook{
		controller:              options.Controller,
		scopeWatchesToNamespace: options.ScopeWatchesToNamespace,
		labelMaker:              options.LabelMaker,
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error building dynamic kubernetes client: %w", err)
	}

	dw, events, err := watch.NewDynamicWatch(restMapper, client)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic watch: %v", err)
	}
	src := &source.Channel{Source: events}
	src.InjectStopChannel(stopChannel)
	if err := options.Controller.Watch(src, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, fmt.Errorf("setting up dynamic watch on the controller: %w", err)
	}

	afterApplyHook.local = &clusterWatch{
		dw:                      dw,
		scopeWatchesToNamespace: options.ScopeWatchesToNamespace,
		labelMaker:              options.LabelMaker,
		registered:              make(map[string]struct{}),
	}

	options.Reconciler.AddHook(afterApplyHook)

	return stopChannel, nil
}

// afterApplyHookk implements the after-apply hook to add watches to our objects
type afterApplyHook struct {
	local *clusterWatch

	// LabelMaker is used to build the labels we should watch on.
	labelMaker LabelMaker

	// Controller contains the controller itself
	controller controller.Controller

	// ScopeWatchesToNamespace controls whether watches are per-namespace.
	// This allows for more narrowly scoped RBAC permissions, at the cost of more watches.
	scopeWatchesToNamespace bool

	mutex         sync.Mutex
	remoteTargets map[string]*clusterWatch
}

// clusterWatch watches the objects in one cluster
type clusterWatch struct {
	dw DynamicWatch

	scopeWatchesToNamespace bool
	labelMaker              LabelMaker

	mutex sync.Mutex

	// registered tracks the objects we are currently watching, avoiding duplicate watches.
	registered map[string]struct{}
}

var _ AfterApply = &afterApplyHook{}

// AfterApply is called by the controller after an apply.  We establish any new watches.
func (w *afterApplyHook) AfterApply(ctx context.Context, op *ApplyOperation) error {
	if op.Target == nil || op.Target.ClusterKey() == target.LocalClusterKey {
		return w.local.afterApply(ctx, op)
	}

	remoteTargetKey := op.Target.ClusterKey()

	w.mutex.Lock()
	defer w.mutex.Unlock()

	remoteClusterWatch := w.remoteTargets[remoteTargetKey]
	if remoteClusterWatch == nil {
		// Inject a stop channel that will never close. The controller does not have a concept of
		// shutdown, so there is no opportunity to stop the watch.
		stopChannel := make(chan struct{})

		restConfig := op.Target.RESTConfig()
		restMapper := op.Target.RESTMapper()

		client, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("error building dynamic kubernetes client: %w", err)
		}

		dw, events, err := watch.NewDynamicWatch(restMapper, client)
		if err != nil {
			return fmt.Errorf("creating dynamic watch: %v", err)
		}
		src := &source.Channel{Source: events}
		src.InjectStopChannel(stopChannel)
		if err := w.controller.Watch(src, &handler.EnqueueRequestForObject{}); err != nil {
			return fmt.Errorf("setting up dynamic watch on the controller: %w", err)
		}

		remoteClusterWatch = &clusterWatch{
			dw:                      dw,
			scopeWatchesToNamespace: w.scopeWatchesToNamespace,
			labelMaker:              w.labelMaker,
			registered:              make(map[string]struct{}),
		}

		if w.remoteTargets == nil {
			w.remoteTargets = make(map[string]*clusterWatch)
		}
		w.remoteTargets[remoteTargetKey] = remoteClusterWatch
	}
	return remoteClusterWatch.afterApply(ctx, op)
}

func (w *clusterWatch) afterApply(ctx context.Context, op *ApplyOperation) error {
	log := log.FromContext(ctx)

	labelSelector, err := labels.ValidatedSelectorFromSet(w.labelMaker(ctx, op.Subject))
	if err != nil {
		return fmt.Errorf("failed to build label selector: %w", err)
	}

	notify := metav1.ObjectMeta{Name: op.Subject.GetName(), Namespace: op.Subject.GetNamespace()}
	filter := metav1.ListOptions{LabelSelector: labelSelector.String()}

	// Protect against concurrent invocation
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, obj := range op.Objects.Items {
		gvk := obj.GroupVersionKind()

		key := fmt.Sprintf("gvk=%s:%s:%s;labels=%s", gvk.Group, gvk.Version, gvk.Kind, filter.LabelSelector)

		filterNamespace := ""
		if w.scopeWatchesToNamespace && obj.GetNamespace() != "" {
			filterNamespace = obj.GetNamespace()
			key += ";namespace=" + filterNamespace
		}

		if _, found := w.registered[key]; found {
			continue
		}

		log.Info("adding watch", "key", key)
		err := w.dw.Add(gvk, filter, filterNamespace, notify)
		if err != nil {
			log.WithValues("GroupVersionKind", gvk.String()).Error(err, "adding watch")
			continue
		}

		w.registered[key] = struct{}{}
	}
	return nil
}
