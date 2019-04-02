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

package dashboard

import (
	//applicationv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	api "sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator/pkg/apis/addons/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
)

var _ reconcile.Reconciler = &ReconcileDashboard{}

// ReconcileDashboard reconciles a Dashboard object
type ReconcileDashboard struct {
	declarative.Reconciler
}

// for WithApplyPrune
// +kubebuilder:rbac:groups=*,resources=*,verbs=list

// +kubebuilder:rbac:groups=addons.sigs.k8s.io,resources=dashboards,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts;secrets,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;delete;patch

// RBAC roles that need to be granted:
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=create
// TODO: can be scoped to resourceNames: ["kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs"]
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;update;delete
// TODO: can be scoped to resourceNames: ["kubernetes-dashboard-settings"]
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;update;delete
// TODO: can be scoped to resourceNames: ["heapster"]
// +kubebuilder:rbac:groups="",resources=services,verbs=proxy
// TODO: can be scoped to resourceNames: ["heapster", "http:heapster:", "https:heapster:"], verbs: ["get"]
// +kubebuilder:rbac:groups="",resources=services/proxy,verbs=get

// newReconciler returns a initialized ReconcileDashboard
func newReconciler(mgr manager.Manager) (r *ReconcileDashboard, srcLabels declarative.LabelMaker, err error) {
	labels := map[string]string{
		"k8s-app": "kubernetes-dashboard",
	}

	r = &ReconcileDashboard{}
	srcLabels = declarative.SourceLabel(mgr.GetScheme())

	err = r.Reconciler.Init(mgr, &api.Dashboard{},
		declarative.WithObjectTransform(declarative.AddLabels(labels)),
		declarative.WithOwner(declarative.SourceAsOwner),
		declarative.WithLabels(srcLabels),
		declarative.WithStatus(status.NewBasic(mgr.GetClient())),
		declarative.WithPreserveNamespace(),
		declarative.WithApplyPrune(),
		declarative.WithManagedApplication(srcLabels),
		declarative.WithObjectTransform(addon.TransformApplicationFromStatus),
	)

	return
}

func Add(mgr manager.Manager) error {
	r, srcLabels, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	c, err := controller.New("dashboard-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Dashboard
	err = c.Watch(&source.Kind{Type: &api.Dashboard{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to deployed objects
	_, err = declarative.WatchAll(mgr.GetConfig(), c, r, srcLabels)
	if err != nil {
		return err
	}

	return nil
}
