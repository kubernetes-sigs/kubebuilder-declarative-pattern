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
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	recorder "k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/utils"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/applier"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target"
)

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	prototype DeclarativeObject

	localClusterTarget *target.Cluster

	client client.Client
	// config *rest.Config

	metrics reconcileMetrics
	mgr     manager.Manager

	// recorder is the EventRecorder for creating k8s events
	recorder recorder.EventRecorder
	// dynamicClient dynamic.Interface

	// restMapper meta.RESTMapper
	options reconcilerParams
}

type DeclarativeObject interface {
	runtime.Object
	metav1.Object
}

// Pruner is a trait for addon CRDs that determines whether pruning behavior should be enabled for the current CR.
// To enable this feature, it's necessary to enable WithApplyPrune. If WithApplyPrune is enabled but Pruner is not
// implemented, Prune behavior is assumed by default.
type Pruner interface {
	Prune() bool
}

// PruneWhiteLister is a trait for addon CRDs that determines which kind of resources should be pruned. It's useful
// when CR in installed by Addon and want to prune them automatically. The format of array item should be exactly like
// <group>/<version>/<kind> (core group using 'core' indeed). For example: ["core/v1/ConfigMap", "batch/v1/Job"].
// Notice: kubeadm has a built-in prune white list, and it will be ignored if this method is implemented.
type PruneWhiteLister interface {
	PruneWhiteList() []string
}

type ErrorResult struct {
	Result reconcile.Result
	Err    error
}

func (e *ErrorResult) Error() string {
	return e.Err.Error()
}

// For mocking
var defaultApplier = applier.NewDirectApplier()

func (r *Reconciler) Init(mgr manager.Manager, prototype DeclarativeObject, opts ...ReconcilerOption) error {
	r.prototype = prototype

	// TODO: Can we derive the name from prototype?
	controllerName := "addon-controller"
	r.recorder = mgr.GetEventRecorderFor(controllerName)

	r.client = mgr.GetClient()

	r.mgr = mgr
	globalObjectTracker.mgr = mgr

	if err := r.applyOptions(opts...); err != nil {
		return err
	}

	if r.options.cache == nil {
		cache, err := target.NewCache(mgr.GetConfig(), mgr.GetRESTMapper())
		if err != nil {
			return err
		}
		r.options.cache = cache
	}
	r.localClusterTarget = r.options.cache.LocalCluster()

	if err := r.validateOptions(); err != nil {
		return err
	}

	if r.CollectMetrics() {
		if gvk, err := apiutil.GVKForObject(prototype, r.mgr.GetScheme()); err != nil {
			return err
		} else {
			r.metrics = reconcileMetricsFor(gvk)
		}
	}

	return nil
}

// +rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	var objects *manifest.Objects

	log := log.FromContext(ctx)
	defer r.collectMetrics(request, result, err)

	// Fetch the object
	instance := r.prototype.DeepCopyObject().(DeclarativeObject)
	if err = r.client.Get(ctx, request.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "error reading object")
		return reconcile.Result{}, err
	}

	// status.Reconciled should catch all error
	defer func() {
		// error is data
		resultErr, ok := err.(*ErrorResult)
		if ok {
			result = resultErr.Result
			err = resultErr.Err
		}

		if r.options.status != nil {
			if statusErr := r.options.status.Reconciled(ctx, instance, objects, err); statusErr != nil {
				log.Error(statusErr, "failed to reconcile status")
			}
		}
	}()

	original := instance.DeepCopyObject().(DeclarativeObject)

	if r.options.status != nil {
		if err := r.options.status.Preflight(ctx, instance); err != nil {
			log.Error(err, "preflight check failed, not reconciling")
			return reconcile.Result{}, err
		}
	}

	var statusInfo *StatusInfo
	statusInfo, err = r.reconcileExists(ctx, request.NamespacedName, instance)
	objects = statusInfo.Manifest // for the defer block
	if err != nil {
		statusInfo.Err = err
	}

	if err != nil {
		r.recorder.Eventf(instance, "Warning", "InternalError", "internal error: %v", err)
	}
	if err != nil {
		statusInfo.Err = err
	}

	// error includes result (for example a retry)
	resultErr, ok := err.(*ErrorResult)
	if ok {
		result = resultErr.Result
		err = resultErr.Err
	}

	if r.options.status != nil {
		if err := r.options.status.BuildStatus(ctx, statusInfo); err != nil {
			log.Error(err, "error building status")
			return result, err
		}
	}

	// Write the status if it has changed
	oldStatus, err := utils.GetCommonStatus(original)
	if err != nil {
		log.Error(err, "error getting status")
		return result, err
	}
	newStatus, err := utils.GetCommonStatus(instance)
	if err != nil {
		log.Error(err, "error getting status")
		return result, err
	}
	if !reflect.DeepEqual(oldStatus, newStatus) {
		if err := r.client.Status().Update(ctx, instance); err != nil {
			log.Error(err, "error updating status")
			return result, err
		}
	}

	return result, err
}

func (r *Reconciler) reconcileExists(ctx context.Context, name types.NamespacedName, instance DeclarativeObject) (*StatusInfo, error) {
	log := log.FromContext(ctx)
	log.WithValues("object", name.String()).Info("reconciling")

	statusInfo := &StatusInfo{
		Subject: instance,
	}

	var fs filesys.FileSystem
	if r.IsKustomizeOptionUsed() {
		fs = filesys.MakeFsInMemory()
	}

	objects, err := r.BuildDeploymentObjectsWithFs(ctx, name, instance, fs)
	if err != nil {
		log.Error(err, "building deployment objects")
		return statusInfo, fmt.Errorf("error building deployment objects: %v", err)
	}
	statusInfo.Manifest = objects
	log.WithValues("objects", fmt.Sprintf("%d", len(objects.Items))).Info("built deployment objects")

	if r.options.status != nil {
		isValidVersion, err := r.options.status.VersionCheck(ctx, instance, objects)
		if err != nil {
			if !isValidVersion {
				statusInfo.KnownError = KnownErrorVersionCheckFailed
				r.recorder.Event(instance, "Warning", "Failed version check", err.Error())
				log.Error(err, "Version check failed, not reconciling")
				return statusInfo, nil
			} else {
				log.Error(err, "Version check failed")
				return statusInfo, err
			}
		}
	}

	objects, err = parseListKind(objects)
	if err != nil {
		log.Error(err, "Parsing list kind")
		return statusInfo, fmt.Errorf("error parsing list kind: %v", err)
	}
	statusInfo.Manifest = objects

	target := r.localClusterTarget

	err = r.setNamespaces(ctx, target, instance, objects)
	if err != nil {
		return statusInfo, err
	}

	err = r.injectOwnerRef(ctx, instance, objects)
	if err != nil {
		return statusInfo, err
	}

	var newItems []*manifest.Object
	for _, obj := range objects.Items {

		unstruct, err := GetObjectFromCluster(ctx, target, obj, r)
		if err != nil && !apierrors.IsNotFound(errors.Unwrap(err)) {
			log.WithValues("name", obj.GetName()).Error(err, "Unable to get resource")
		}
		if unstruct != nil {
			annotations := unstruct.GetAnnotations()
			if _, ok := annotations["addons.k8s.io/ignore"]; ok {
				log.WithValues("kind", obj.Kind).WithValues("name", obj.GetName()).Info("Found ignore annotation on object, " +
					"skipping object")
				continue
			}
		}
		newItems = append(newItems, obj)
	}
	objects.Items = newItems

	extraArgs := []string{}

	// allow user disable prune in CR
	if p, ok := instance.(Pruner); (!ok && r.options.prune) || (ok && r.options.prune && p.Prune()) {
		var labels []string
		for k, v := range r.options.labelMaker(ctx, instance) {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		extraArgs = append(extraArgs, "--prune", "--selector", strings.Join(labels, ","))

		if lister, ok := instance.(PruneWhiteLister); ok {
			for _, gvk := range lister.PruneWhiteList() {
				extraArgs = append(extraArgs, "--prune-whitelist", gvk)
			}
		}
	}

	ns := ""
	if !r.options.preserveNamespace {
		ns = name.Namespace
	}

	if r.CollectMetrics() {
		if errs := globalObjectTracker.addIfNotPresent(objects.Items, ns); errs != nil {
			for _, err := range errs.Errors() {
				if errors.Is(err, noRESTMapperErr{}) {
					log.WithName("declarative_reconciler").Error(err, "failed to get corresponding RESTMapper from API server")
				} else if errors.Is(err, emptyNamespaceErr{}) {
					// There should be no route to this path
					log.WithName("declarative_reconciler").Info("Scoped object, but no namespace specified")
				} else {
					log.WithName("declarative_reconciler").Error(err, "Unknown error")
				}
			}
		}
	}

	applierOptions := applier.ApplierOptions{
		RESTConfig: target.RESTConfig(),
		RESTMapper: target.RESTMapper(),
		Namespace:  ns,
		Objects:    objects.GetItems(),
		Validate:   r.options.validate,
		ExtraArgs:  extraArgs,
		Force:      true,
		// TODO Make this configurable
		CascadingStrategy: "Foreground",
	}

	applyOperation := &ApplyOperation{
		Subject:        instance,
		Objects:        objects,
		ApplierOptions: &applierOptions,
	}

	applier := r.options.applier
	for _, hook := range r.options.hooks {
		if beforeApply, ok := hook.(BeforeApply); ok {
			if err := beforeApply.BeforeApply(ctx, applyOperation); err != nil {
				log.Error(err, "calling BeforeApply hook")
				return statusInfo, fmt.Errorf("error calling BeforeApply hook: %v", err)
			}
		}
	}

	if err := applier.Apply(ctx, *applyOperation.ApplierOptions); err != nil {
		log.Error(err, "applying manifest")
		statusInfo.KnownError = KnownErrorApplyFailed
		return statusInfo, fmt.Errorf("error applying manifest: %v", err)
	}

	statusInfo.LiveObjects = func(ctx context.Context, gvk schema.GroupVersionKind, nn types.NamespacedName) (*unstructured.Unstructured, error) {
		// TODO: Applier should return the objects in their post-apply state, so we don't have to requery
		mapping, err := target.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("unable to get mapping for resource %v: %w", gvk, err)
		}

		var resource dynamic.ResourceInterface
		switch mapping.Scope {
		case meta.RESTScopeNamespace:
			resource = target.DynamicClient().Resource(mapping.Resource).Namespace(nn.Namespace)
		case meta.RESTScopeRoot:
			resource = target.DynamicClient().Resource(mapping.Resource)
		default:
			return nil, fmt.Errorf("unknown scope %v", mapping.Scope)
		}
		u, err := resource.Get(ctx, nn.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting object: %w", err)
		}
		return u, nil

	}

	if r.options.sink != nil {
		if err := r.options.sink.Notify(ctx, instance, objects); err != nil {
			log.Error(err, "notifying sink")
			return statusInfo, err
		}
	}

	for _, hook := range r.options.hooks {
		if afterApply, ok := hook.(AfterApply); ok {
			if err := afterApply.AfterApply(ctx, applyOperation); err != nil {
				log.Error(err, "calling AfterApply hook")
				return statusInfo, fmt.Errorf("error calling AfterApply hook: %w", err)
			}
		}
	}

	return statusInfo, nil
}

// BuildDeploymentObjects performs all manifest operations to build a final set of objects for deployment
func (r *Reconciler) BuildDeploymentObjects(ctx context.Context, name types.NamespacedName, instance DeclarativeObject) (*manifest.Objects, error) {
	return r.BuildDeploymentObjectsWithFs(ctx, name, instance, nil)
}

// BuildDeploymentObjectsWithFs is the implementation of BuildDeploymentObjects, supporting saving to a filesystem for kustomize
// If fs is provided, the transformed manifests will be saved to that filesystem
func (r *Reconciler) BuildDeploymentObjectsWithFs(ctx context.Context, name types.NamespacedName, instance DeclarativeObject, fs filesys.FileSystem) (*manifest.Objects, error) {
	log := log.FromContext(ctx)

	// 1. Load the manifest
	manifestFiles, err := r.loadRawManifest(ctx, instance)
	if err != nil {
		log.Error(err, "error loading raw manifest")
		return nil, err
	}
	manifestObjects := &manifest.Objects{}
	// 2. Perform raw string operations
	for manifestPath, manifestStr := range manifestFiles {
		for _, t := range r.options.rawManifestOperations {
			transformed, err := t(ctx, instance, manifestStr)
			if err != nil {
				log.Error(err, "error performing raw manifest operations")
				return nil, err
			}
			manifestStr = transformed
		}

		// 3. Parse manifest into objects
		objects, err := r.parseManifest(ctx, instance, manifestStr)
		if err != nil {
			log.Error(err, "error parsing manifest")
			return nil, err
		}

		// 4. Perform object transformations
		// (unless kustomize is in use, in which case we transform after running kustomize)
		if !r.IsKustomizeOptionUsed() {
			if err := r.transformManifest(ctx, instance, objects); err != nil {
				log.Error(err, "error transforming manifest")
				return nil, err
			}
		}

		if fs != nil {
			// 5. Write objects to filesystem for kustomizing, allow multiple objects in a file
			finalJson := []byte("")
			separator := []byte("---\n")
			for _, item := range objects.Items {
				json, err := item.JSON()
				if err != nil {
					log.Error(err, "error converting object to json")
					return nil, err
				}
				finalJson = append(finalJson, separator...)
				finalJson = append(finalJson, json...)
			}
			fs.WriteFile(string(manifestPath), finalJson)
			for _, blob := range objects.Blobs {
				fs.WriteFile(string(manifestPath), blob)
			}
		}
		manifestObjects.Path = filepath.Dir(manifestPath)
		manifestObjects.Items = append(manifestObjects.Items, objects.Items...)
		manifestObjects.Blobs = append(manifestObjects.Blobs, objects.Blobs...)
	}

	// If Kustomize option is on, it's assumed that the entire addon manifest is created using Kustomize
	// Here, the manifest is built using Kustomize and then replaces the Object items with the created manifest
	if r.IsKustomizeOptionUsed() {
		// run kustomize to create final manifest
		opts := krusty.MakeDefaultOptions()
		k := krusty.MakeKustomizer(opts)
		m, err := k.Run(fs, manifestObjects.Path)
		if err != nil {
			log.Error(err, "running kustomize to create final manifest")
			return nil, fmt.Errorf("error running kustomize: %v", err)
		}

		manifestYaml, err := m.AsYaml()
		if err != nil {
			log.Error(err, "creating final manifest yaml")
			return nil, fmt.Errorf("error converting kustomize output to yaml: %v", err)
		}

		objects, err := r.parseManifest(ctx, instance, string(manifestYaml))
		if err != nil {
			log.Error(err, "creating final manifest yaml")
			return nil, err
		}

		if err := r.transformManifest(ctx, instance, objects); err != nil {
			log.Error(err, "error transforming manifest")
			return nil, err
		}
		manifestObjects.Items = objects.Items
	}

	// 6. Sort objects to work around dependent objects in the same manifest (eg: service-account, deployment)
	manifestObjects.Sort(DefaultObjectOrder(ctx))

	return manifestObjects, nil
}

// parseManifest parses the manifest into objects
func (r *Reconciler) parseManifest(ctx context.Context, instance DeclarativeObject, manifestStr string) (*manifest.Objects, error) {
	log := log.FromContext(ctx)

	objects, err := manifest.ParseObjects(ctx, manifestStr)
	if err != nil {
		log.Error(err, "error parsing manifest")
		return nil, err
	}

	return objects, nil
}

// transformManifest runs any transformations as required
func (r *Reconciler) transformManifest(ctx context.Context, instance DeclarativeObject, objects *manifest.Objects) error {
	transforms := r.options.objectTransformations
	if r.options.labelMaker != nil {
		transforms = append(transforms, AddLabels(r.options.labelMaker(ctx, instance)))
	}
	// TODO(jrjohnson): apply namespace here
	for _, t := range transforms {
		err := t(ctx, instance, objects)
		if err != nil {
			return err
		}
	}
	return nil
}

// loadRawManifest loads the raw manifest YAML from the repository
func (r *Reconciler) loadRawManifest(ctx context.Context, o DeclarativeObject) (map[string]string, error) {
	s, err := r.options.manifestController.ResolveManifest(ctx, o)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (r *Reconciler) applyOptions(opts ...ReconcilerOption) error {
	params := reconcilerParams{}

	params.applier = defaultApplier

	opts = append(Options.Begin, opts...)
	opts = append(opts, Options.End...)

	for _, opt := range opts {
		params = opt(params)
	}

	// Default the manifest controller if not set
	if params.manifestController == nil && DefaultManifestLoader != nil {
		loader, err := DefaultManifestLoader()
		if err != nil {
			return err
		}
		params.manifestController = loader
	}

	r.options = params
	return nil
}

// Validate compatibility of selected options
func (r *Reconciler) validateOptions() error {
	var errs []string

	if r.options.prune && r.options.labelMaker == nil {
		errs = append(errs, "WithApplyPrune must be used with the WithLabels option")
	}

	if r.options.manifestController == nil {
		errs = append(errs, "ManifestController must be set either by configuring DefaultManifestLoader or specifying the WithManifestController option")
	}

	if len(errs) != 0 {
		return fmt.Errorf(strings.Join(errs, ","))
	}

	return nil
}

// setNamespaces will set the object on all namespace-scoped objects, unless the preserveNamespace option is set
func (r *Reconciler) setNamespaces(ctx context.Context, cluster *target.Cluster, instance DeclarativeObject, objects *manifest.Objects) error {
	if r.options.preserveNamespace {
		return nil
	}

	ns := instance.GetNamespace()
	if ns == "" {
		// No namespace to set
		return nil
	}

	log := log.FromContext(ctx)
	log.WithValues("namespace", ns).Info("setting namespace")

	for _, o := range objects.Items {
		if o.GetNamespace() != "" {
			continue
		}

		gvk := o.GroupVersionKind()
		mapping, err := cluster.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			log.Error(err, "error getting scope for gvk", "gvk", gvk)
			continue
		}
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			o.SetNamespace(ns)
		}
	}
	return nil
}

func (r *Reconciler) injectOwnerRef(ctx context.Context, instance DeclarativeObject, objects *manifest.Objects) error {
	if r.options.ownerFn == nil {
		return nil
	}

	log := log.FromContext(ctx)
	log.WithValues("object", fmt.Sprintf("%s/%s", instance.GetName(), instance.GetNamespace())).Info("injecting owner references")

	for _, o := range objects.Items {
		owner, err := r.options.ownerFn(ctx, instance, *o, *objects)
		if err != nil {
			log.WithValues("object", o).Error(err, "resolving owner ref", o)
			return err
		}
		if owner == nil {
			log.WithValues("object", o).Info("no owner resolved")
			continue
		}
		if owner.GetName() == "" {
			log.WithValues("object", o).Info("has no name")
			continue
		}
		if string(owner.GetUID()) == "" {
			log.WithValues("object", o).Info("has no UID")
			continue
		}

		gvk, err := apiutil.GVKForObject(owner, r.mgr.GetScheme())
		if err != nil {
			log.WithValues("object", o).Error(err, "unable to get GVK for object")
			continue
		}
		if gvk.Group == "" || gvk.Version == "" {
			log.WithValues("object", o).WithValues("GroupVersionKind", gvk).Info("is not valid")
			continue
		}

		if owner.GetNamespace() != "" && owner.GetNamespace() != o.GetNamespace() {
			// a namespaced object can only own objects within the same namespace, not objects in other namespaces or cluster-scoped objects
			// for any other combination, skip setting owner reference here, to allow declarative.SourceAsOwner to be used for the
			// subset of objects that make up a supported combination
			log.WithValues("object", o).Info("not setting ownerRef across namespaces")
			continue
		}

		ownerRefs := []interface{}{
			map[string]interface{}{
				"apiVersion":         gvk.Group + "/" + gvk.Version,
				"blockOwnerDeletion": true,
				"controller":         true,
				"kind":               gvk.Kind,
				"name":               owner.GetName(),
				"uid":                string(owner.GetUID()),
			},
		}
		if err := o.SetNestedField(ownerRefs, "metadata", "ownerReferences"); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) collectMetrics(request reconcile.Request, result reconcile.Result, err error) {
	if r.options.metrics {
		r.metrics.reconcileWith(request)
		r.metrics.reconcileFailedWith(request, result, err)
	}
}

// IsKustomizeOptionUsed checks if the option for Kustomize build is used for creating manifests
func (r *Reconciler) IsKustomizeOptionUsed() bool {
	if r.options.kustomize {
		return true
	}
	return false
}

// SetSink provides a Sink that will be notified for all deployments
//
// Deprecated: prefer WithHook
func (r *Reconciler) SetSink(sink Sink) {
	r.options.sink = sink
}

// AddHook provides a Hook that will be notified of significant events
func (r *Reconciler) AddHook(hook Hook) {
	r.options.hooks = append(r.options.hooks, hook)
}

func parseListKind(infos *manifest.Objects) (*manifest.Objects, error) {
	var out []*manifest.Object

	for _, item := range infos.Items {
		if item.Group == "v1" && item.Kind == "List" {
			itemObj := item.UnstructuredObject()

			err := itemObj.EachListItem(func(obj runtime.Object) error {
				itemUnstructured := obj.(*unstructured.Unstructured)
				newObj, err := manifest.NewObject(itemUnstructured)
				if err != nil {
					return err
				}
				out = append(out, newObj)
				return nil
			})

			if err != nil {
				return nil, err
			}
		} else {
			out = append(out, item)
		}
	}

	ret := manifest.Objects{
		Items: out,
		Blobs: infos.Blobs,
		Path:  infos.Path,
	}

	return &ret, nil
}

// CollectMetrics determines whether metrics of declarative reconciler is enabled
func (r *Reconciler) CollectMetrics() bool {
	return r.options.metrics
}

// GetObjectFromCluster gets the current state of the object from the cluster.
//
// deprecated: use LiveObjectsFunc instead when computing status
func GetObjectFromCluster(ctx context.Context, cluster *target.Cluster, obj *manifest.Object, r *Reconciler) (*unstructured.Unstructured, error) {
	getOptions := metav1.GetOptions{}
	gvk := obj.GroupVersionKind()

	mapping, err := cluster.RESTMapper().RESTMapping(obj.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to get mapping for resource %v: %w", gvk, err)
	}

	ns, name := "", obj.GetName()
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns = obj.GetNamespace()
	}
	unstruct, err := cluster.DynamicClient().Resource(mapping.Resource).Namespace(ns).Get(ctx, name, getOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to get object: %w", err)
	}
	return unstruct, nil
}
