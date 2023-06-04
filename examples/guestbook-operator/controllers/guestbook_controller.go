/*
Copyright 2020 The Kubernetes Authors.

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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"

	api "sigs.k8s.io/kubebuilder-declarative-pattern/examples/guestbook-operator/api/v1alpha1"
)

var _ reconcile.Reconciler = &GuestbookReconciler{}

// GuestbookReconciler reconciles a Guestbook object
type GuestbookReconciler struct {
	declarative.Reconciler
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	watchLabels declarative.LabelMaker
}

func (r *GuestbookReconciler) setupReconciler(mgr ctrl.Manager) error {
	labels := map[string]string{
		"example-app": "guestbook",
	}

	r.watchLabels = declarative.SourceLabel(mgr.GetScheme())

	return r.Reconciler.Init(mgr, &api.Guestbook{},
		declarative.WithObjectTransform(declarative.AddLabels(labels)),
		declarative.WithOwner(declarative.SourceAsOwner),
		declarative.WithLabels(r.watchLabels),
		declarative.WithStatus(status.NewBasic(mgr.GetClient())),
		declarative.WithApplyPrune(),
		declarative.WithObjectTransform(addon.ApplyPatches),
		declarative.WithApplyKustomize(),

		// Add other optional options for testing
		declarative.WithApplyValidation(),
		declarative.WithReconcileMetrics(0, nil),
	)
}

func (r *GuestbookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.setupReconciler(mgr); err != nil {
		return err
	}

	c, err := controller.New("guestbook-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Guestbook
	err = c.Watch(source.Kind(mgr.GetCache(), &api.Guestbook{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to deployed objects
	err = declarative.WatchChildren(declarative.WatchChildrenOptions{Manager: mgr, Controller: c, Reconciler: r, LabelMaker: r.watchLabels})
	if err != nil {
		return err
	}

	return nil
}

// for WithApplyPrune
// +kubebuilder:rbac:groups=*,resources=*,verbs=list

// +kubebuilder:rbac:groups=addons.example.org,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=addons.example.org,resources=guestbooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments,verbs=get;list;watch;create;update;delete;patch
