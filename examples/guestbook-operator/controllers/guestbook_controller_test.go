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
	"path/filepath"
	"testing"

	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	api "sigs.k8s.io/kubebuilder-declarative-pattern/examples/guestbook-operator/api/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/golden"
)

func TestGuestbook(t *testing.T) {
	log.SetLogger(klogr.New())

	env := &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			ErrorIfPathMissing: true,
			Paths: []string{
				filepath.Join("..", "config", "crd", "bases"),
			},
		},
	}
	opt := golden.ValidatorOptions{
		EnvtestEnvironment: env,
		ManagerOptions: manager.Options{
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
		},
	}
	opt.WithSchema(api.AddToScheme)

	v := golden.NewValidator(t, opt)

	v.Validate(func(mgr manager.Manager) (*declarative.Reconciler, error) {
		gr := &GuestbookReconciler{
			Client: mgr.GetClient(),
		}
		err := gr.setupReconciler(mgr)
		if err != nil {
			return nil, err
		}
		return &gr.Reconciler, nil
	})
}
