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
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/examples/metrics-server-operator/api/v1alpha1"
)

// MetricsServerReconciler reconciles a MetricsServer object
type MetricsServerReconciler struct {
	declarative.Reconciler
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	watchLabels declarative.LabelMaker
}

var _ reconcile.Reconciler = &MetricsServerReconciler{}

// +kubebuilder:rbac:groups=addons.example.org,resources=metricsservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=addons.example.org,resources=metricsservers/status,verbs=get;update;patch

func (r *MetricsServerReconciler) setupReconciler(mgr ctrl.Manager) error {
	labels := map[string]string{
		"k8s-app": "metrics-server",
	}
	r.watchLabels = declarative.SourceLabel(mgr.GetScheme())

	return r.Reconciler.Init(mgr, &addonsv1alpha1.MetricsServer{},
		declarative.WithObjectTransform(declarative.AddLabels(labels)),
		declarative.WithOwner(declarative.SourceAsOwner),
		declarative.WithLabels(r.watchLabels),
		declarative.WithStatus(status.NewBasic(mgr.GetClient())),
		declarative.WithPreserveNamespace(),
		declarative.WithApplyPrune(),
		declarative.WithObjectTransform(addon.ApplyPatches),
	)
}

func (r *MetricsServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.setupReconciler(mgr); err != nil {
		return err
	}

	c, err := controller.New("metricsserver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MetricsServer
	err = c.Watch(&source.Kind{Type: &addonsv1alpha1.MetricsServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for change to deployed objects
	_, err = declarative.WatchAll(mgr.GetConfig(), c, r, r.watchLabels)
	if err != nil {
		return err
	}

	return nil
}
