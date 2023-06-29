/*
Copyright 2023 TODO(justinsb): assign copyright.

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

package controllers

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kubebuilder-declarative-pattern/applylib/watchset"
	"sigs.k8s.io/kubebuilder-declarative-pattern/commonclient"
	"sigs.k8s.io/kubebuilder-declarative-pattern/composite/api/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/composite/pkg/engines/manifestengine"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/applier"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

var _ reconcile.Reconciler = &instanceReconciler{}

// instanceReconciler reconciles a CompositeDefinition object
type instanceReconciler struct {
	client        client.Client
	restMapper    meta.RESTMapper
	config        *rest.Config
	dynamicClient dynamic.Interface

	// subject *addonsv1alpha1.CompositeDefinition

	fileName   string
	engine     string
	definition string

	gvk             schema.GroupVersionKind
	watchsetManager *watchset.ControllerManager
}

func (r *instanceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := klog.FromContext(ctx)

	id := req.NamespacedName
	subject := &unstructured.Unstructured{}
	subject.SetAPIVersion(r.gvk.GroupVersion().Identifier())
	subject.SetKind(r.gvk.Kind)

	if err := r.client.Get(ctx, id, subject); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log.Info("reconcile request for object", "id", id)

	result, err := r.reconcileExists(ctx, id, subject)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Info("result", "result", result)
	return reconcile.Result{}, err
}

type Engine interface {
	BuildObjects(ctx context.Context, fileName string, script string, subject *unstructured.Unstructured) ([]*unstructured.Unstructured, error)
}

func (r *instanceReconciler) BuildDeploymentObjects(ctx context.Context, name types.NamespacedName, subject *unstructured.Unstructured) (*manifest.Objects, error) {
	var engine Engine
	switch r.engine {
	case "yaml":
		engine = manifestengine.NewEngine(r.restMapper, r.dynamicClient)
	default:
		return nil, fmt.Errorf("engine %q not known", r.engine)
	}

	objects, err := engine.BuildObjects(ctx, r.fileName, r.definition, subject)
	if err != nil {
		return nil, err
	}
	out := &manifest.Objects{}
	for _, u := range objects {
		o, err := manifest.NewObject(u)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, o)
	}
	return out, nil
}

func (r *instanceReconciler) reconcileExists(ctx context.Context, name types.NamespacedName, instance *unstructured.Unstructured) (*declarative.StatusInfo, error) {
	log := log.FromContext(ctx)
	log.WithValues("object", name.String()).Info("reconciling")

	statusInfo := &declarative.StatusInfo{}
	statusInfo.Subject = instance

	// var fs filesys.FileSystem
	// if r.IsKustomizeOptionUsed() {
	// 	fs = filesys.MakeFsInMemory()
	// }

	// objects, err := r.BuildDeploymentObjectsWithFs(ctx, name, instance, fs)
	objects, err := r.BuildDeploymentObjects(ctx, name, instance)
	if err != nil {
		log.Error(err, "building deployment objects")
		return statusInfo, fmt.Errorf("error building deployment objects: %v", err)
	}

	// objects, err = flattenListObjects(objects)
	// if err != nil {
	// 	log.Error(err, "flattening list objects")
	// 	return statusInfo, fmt.Errorf("error flattening list objects: %w", err)
	// }
	log.WithValues("objects", fmt.Sprintf("%d", len(objects.Items))).Info("built deployment objects")
	statusInfo.Manifest = objects

	// if r.options.status != nil {
	// 	isValidVersion, err := r.options.status.VersionCheck(ctx, instance, objects)
	// 	if err != nil {
	// 		if !isValidVersion {
	// 			statusInfo.KnownError = KnownErrorVersionCheckFailed
	// 			r.recorder.Event(instance, "Warning", "Failed version check", err.Error())
	// 			log.Error(err, "Version check failed, not reconciling")
	// 			return statusInfo, nil
	// 		} else {
	// 			log.Error(err, "Version check failed")
	// 			return statusInfo, err
	// 		}
	// 	}
	// }

	// err = r.setNamespaces(ctx, instance, objects)
	// if err != nil {
	// 	return statusInfo, err
	// }

	// err = r.injectOwnerRef(ctx, instance, objects)
	// if err != nil {
	// 	return statusInfo, err
	// }

	// for _, obj := range objects.Items {
	// unstruct, err := GetObjectFromCluster(obj, r)
	// if err != nil && !apierrors.IsNotFound(errors.Unwrap(err)) {
	// 	log.WithValues("name", obj.GetName()).Error(err, "Unable to get resource")
	// }
	// if unstruct != nil {
	// 	annotations := unstruct.GetAnnotations()
	// 	if _, ok := annotations["addons.k8s.io/ignore"]; ok {
	// 		log.WithValues("kind", obj.Kind).WithValues("name", obj.GetName()).Info("Found ignore annotation on object, " +
	// 			"skipping object")
	// 		continue
	// 	}
	// }
	// }

	var newItems []*manifest.Object
	for _, obj := range objects.Items {

		// unstruct, err := GetObjectFromCluster(obj, r)
		// if err != nil && !apierrors.IsNotFound(errors.Unwrap(err)) {
		// 	log.WithValues("name", obj.GetName()).Error(err, "Unable to get resource")
		// }
		// if unstruct != nil {
		// 	annotations := unstruct.GetAnnotations()
		// 	if _, ok := annotations["addons.k8s.io/ignore"]; ok {
		// 		log.WithValues("kind", obj.Kind).WithValues("name", obj.GetName()).Info("Found ignore annotation on object, " +
		// 			"skipping object")
		// 		continue
		// 	}
		// }
		newItems = append(newItems, obj)
	}
	objects.Items = newItems

	extraArgs := []string{}

	// allow user disable prune in CR
	// if p, ok := instance.(Pruner); (!ok && r.options.prune) || (ok && r.options.prune && p.Prune()) {
	// 	var labels []string
	// 	for k, v := range r.options.labelMaker(ctx, instance) {
	// 		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	// 	}

	// 	extraArgs = append(extraArgs, "--prune", "--selector", strings.Join(labels, ","))

	// 	if lister, ok := instance.(PruneWhiteLister); ok {
	// 		for _, gvk := range lister.PruneWhiteList() {
	// 			extraArgs = append(extraArgs, "--prune-whitelist", gvk)
	// 		}
	// 	}
	// }

	ns := ""
	// if !r.options.preserveNamespace {
	ns = name.Namespace
	// }

	// if r.CollectMetrics() {
	// 	if errs := globalObjectTracker.addIfNotPresent(objects.Items, ns); errs != nil {
	// 		for _, err := range errs.Errors() {
	// 			if errors.Is(err, noRESTMapperErr{}) {
	// 				log.WithName("declarative_reconciler").Error(err, "failed to get corresponding RESTMapper from API server")
	// 			} else if errors.Is(err, emptyNamespaceErr{}) {
	// 				// There should be no route to this path
	// 				log.WithName("declarative_reconciler").Info("Scoped object, but no namespace specified")
	// 			} else {
	// 				log.WithName("declarative_reconciler").Error(err, "Unknown error")
	// 			}
	// 		}
	// 	}
	// }

	parentRef, err := applier.NewParentRef(r.restMapper, instance, instance.GetName(), instance.GetNamespace())
	if err != nil {
		return statusInfo, err
	}
	applierOpt := applier.ApplierOptions{
		RESTConfig:        r.config,
		RESTMapper:        r.restMapper,
		Namespace:         ns,
		ParentRef:         parentRef,
		Objects:           objects.GetItems(),
		Validate:          false, //r.options.validate,
		ExtraArgs:         extraArgs,
		Force:             true,
		CascadingStrategy: "Foreground", //r.options.cascadingStrategy,
		Client:            r.client,
	}

	// TODO: Don't prune until objects are healthy
	applierOpt.Prune = true

	// applyOperation := &declarative.ApplyOperation{
	// 	Subject:        instance,
	// 	Objects:        objects,
	// 	ApplierOptions: &applierOpt,
	// }

	// applier := r.options.applier
	// for _, hook := range r.options.hooks {
	// 	if beforeApply, ok := hook.(BeforeApply); ok {
	// 		if err := beforeApply.BeforeApply(ctx, applyOperation); err != nil {
	// 			log.Error(err, "calling BeforeApply hook")
	// 			return statusInfo, fmt.Errorf("error calling BeforeApply hook: %v", err)
	// 		}
	// 	}
	// }

	patchOptions := metav1.PatchOptions{FieldManager: "kdp-test"}

	applier := applier.NewApplySetApplier(patchOptions, metav1.DeleteOptions{}, applier.ApplysetOptions{})

	if err := applier.Apply(ctx, applierOpt); err != nil {
		log.Error(err, "applying manifest")
		statusInfo.KnownError = declarative.KnownErrorApplyFailed
		return statusInfo, fmt.Errorf("error applying manifest: %v", err)
	}

	statusInfo.LiveObjects = func(ctx context.Context, gvk schema.GroupVersionKind, nn types.NamespacedName) (*unstructured.Unstructured, error) {
		// TODO: Applier should return the objects in their post-apply state, so we don't have to requery

		mapping, err := r.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("unable to get mapping for resource %v: %w", gvk, err)
		}

		var resource dynamic.ResourceInterface
		switch mapping.Scope {
		case meta.RESTScopeNamespace:
			resource = r.dynamicClient.Resource(mapping.Resource).Namespace(nn.Namespace)
		case meta.RESTScopeRoot:
			resource = r.dynamicClient.Resource(mapping.Resource)
		default:
			return nil, fmt.Errorf("unknown scope %v", mapping.Scope)
		}
		u, err := resource.Get(ctx, nn.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting object: %w", err)
		}
		return u, nil

	}

	// if r.options.sink != nil {
	// 	if err := r.options.sink.Notify(ctx, instance, objects); err != nil {
	// 		log.Error(err, "notifying sink")
	// 		return statusInfo, err
	// 	}
	// }

	// for _, hook := range r.options.hooks {
	// 	if afterApply, ok := hook.(AfterApply); ok {
	// 		if err := afterApply.AfterApply(ctx, applyOperation); err != nil {
	// 			log.Error(err, "calling AfterApply hook")
	// 			return statusInfo, fmt.Errorf("error calling AfterApply hook: %w", err)
	// 		}
	// 	}
	// }

	return statusInfo, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *instanceReconciler) start(mgr ctrl.Manager) error {
	// addon.Init()

	r.client = mgr.GetClient()
	r.restMapper = mgr.GetRESTMapper()
	r.config = mgr.GetConfig()

	d, err := dynamic.NewForConfig(r.config)
	if err != nil {
		return err
	}
	r.dynamicClient = d

	r.definition = subject.Spec.Definition
	r.engine = subject.Spec.Engine
	r.fileName = subject.GetName()

	r.gvk = schema.FromAPIVersionAndKind(subject.Spec.ReconcilerFor.APIVersion, subject.Spec.ReconcilerFor.Kind)

	// labels := map[string]string{
	// 	"k8s-app": "compositedefinition",
	// }

	// watchLabels := declarative.SourceLabel(mgr.GetScheme())

	// if err := r.Reconciler.Init(mgr, &addonsv1alpha1.CompositeDefinition{},
	// 	declarative.WithObjectTransform(declarative.AddLabels(labels)),
	// 	declarative.WithOwner(declarative.SourceAsOwner),
	// 	declarative.WithLabels(watchLabels),
	// 	declarative.WithStatus(status.NewBasic(mgr.GetClient())),
	// 	// TODO: add an application to your manifest:  declarative.WithObjectTransform(addon.TransformApplicationFromStatus),
	// 	// TODO: add an application to your manifest:  declarative.WithManagedApplication(watchLabels),
	// 	declarative.WithObjectTransform(addon.ApplyPatches),
	// ); err != nil {
	// 	return err
	// }

	// watchLabels := func(ctx context.Context, subject declarative.DeclarativeObject) map[string]string {
	// 	applysetID := subject.GetLabels()["applyset.kubernetes.io/id"]
	// 	if applysetID == "" {
	// 		return nil
	// 	}
	// 	return map[string]string{
	// 		"applyset.kubernetes.io/part-of": applysetID,
	// 	}
	// }

	// // Watch for changes to deployed objects
	// if err := declarative.WatchChildren(declarative.WatchChildrenOptions{Manager: mgr, Controller: c, Reconciler: c, LabelMaker: watchLabels}); err != nil {
	// 	return err
	// }

	return nil
}

type instanceReconcilerRunner struct {
	controller controller.Controller
	reconciler *instanceReconciler
	ctx        context.Context
	cancel     func()
	result     *Future[error]
}

func newInstanceReconcilerRunner(mgr ctrl.Manager, watchsets *watchset.Manager, subject *v1alpha1.CompositeDefinition) (*instanceReconcilerRunner, error) {
	r := &instanceReconciler{}
	if err := r.init(mgr, watchsets, subject); err != nil {
		return nil, err
	}

	c, err := controller.NewUnmanaged("instance-reconciler", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	watchsetManager, err := watchsets.NewControllerManager(c)
	if err != nil {
		return nil, err
	}
	r.watchsetManager = watchsetManager

	actsOn := &unstructured.Unstructured{}
	actsOn.SetAPIVersion(r.gvk.GroupVersion().Identifier())
	actsOn.SetKind(r.gvk.Kind)

	// Watch for changes to CompositeDefinition
	err = c.Watch(commonclient.SourceKind(mgr.GetCache(), actsOn), &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	runner := &instanceReconcilerRunner{
		reconciler: r,
		controller: c,
		ctx:        ctx,
		cancel:     cancel,
		result:     newFuture[error](),
	}

	return runner, nil
}

func (r *instanceReconcilerRunner) stop() error {
	r.reconciler.watchsetManager.Stop()

	r.cancel()

	return r.result.Wait()
}

func (r *instanceReconcilerRunner) start() {
	go func() {
		err := r.controller.Start(r.ctx)
		if err != nil {
			klog.Warningf("error from instance-reconciler controller: %w", err)
		}
		r.result.Set(err)
	}()
}

type Future[T any] struct {
	mutex  sync.Mutex
	cond   *sync.Cond
	done   bool
	result T
}

func newFuture[T any]() *Future[T] {
	f := &Future[T]{}
	f.cond = sync.NewCond(&f.mutex)
	return f
}

func (f *Future[T]) Set(value T) {
	f.mutex.Lock()
	if f.done {
		panic("Future::Set called more than once")
	}
	f.done = true
	f.result = value
	f.cond.Broadcast()
	f.mutex.Unlock()
}

func (f *Future[T]) Poll() (T, bool) {
	f.mutex.Lock()
	result, done := f.result, f.done
	f.mutex.Unlock()
	return result, done
}

func (f *Future[T]) Wait() T {
	f.mutex.Lock()
	for {
		if !f.done {
			f.cond.Wait()
		} else {
			result := f.result
			f.mutex.Unlock()
			return result
		}
	}
}
