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

package status

import (
	"context"
	"fmt"
	"reflect"

	addoncluster "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/cluster"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

const successfulDeployment = appsv1.DeploymentAvailable

// NewAggregator provides an implementation of declarative.Reconciled that
// aggregates the status of deployed objects to configure the 'Healthy'
// field on an addon that derives from CommonStatus
func NewAggregator(client client.Client) *aggregator {
	return &aggregator{client}
}

type aggregator struct {
	client client.Client
}

func (a *aggregator) Reconciled(ctx context.Context, src declarative.DeclarativeObject, objs *manifest.Objects, c addoncluster.Cluster, reconcileErr error) error {
	log := log.FromContext(ctx)

	statusHealthy := true
	statusErrors := []string{}

	if reconcileErr != nil {
		statusHealthy = false
	}

	for _, o := range objs.GetItems() {
		gk := o.Group + "/" + o.Kind
		healthy := true
		objKey := client.ObjectKey{
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		}
		// If the namespace isn't set on the object, we would want to use the namespace of src
		if objKey.Namespace == "" {
			objKey.Namespace = src.GetNamespace()
		}
		var err error
		switch gk {
		case "/Service":
			healthy, err = a.service(ctx, objKey, c)
		case "extensions/Deployment", "apps/Deployment":
			healthy, err = a.deployment(ctx, objKey, c)
		default:
			log.WithValues("type", gk).V(2).Info("type not implemented for status aggregation, skipping")
		}

		statusHealthy = statusHealthy && healthy
		if err != nil {
			statusErrors = append(statusErrors, fmt.Sprintf("%v", err))
		}
	}

	log.WithValues("object", src).WithValues("status", statusHealthy).V(2).Info("built status")

	currentStatus, err := utils.GetCommonStatus(src)
	if err != nil {
		return err
	}

	status := currentStatus
	status.Healthy = statusHealthy
	status.Errors = statusErrors

	if !reflect.DeepEqual(status, currentStatus) {
		err := utils.SetCommonStatus(src, status)
		if err != nil {
			return err
		}

		log.WithValues("name", src.GetName()).WithValues("status", status).Info("updating status")
		err = c.GetClient().Status().Update(ctx, src)
		if err != nil {
			log.Error(err, "updating status")
			return err
		}
	}

	return nil
}

func (a *aggregator) deployment(ctx context.Context, key client.ObjectKey, c addoncluster.Cluster) (bool, error) {
	dep := &appsv1.Deployment{}

	if err := c.GetClient().Get(ctx, key, dep); err != nil {
		return false, fmt.Errorf("error reading deployment (%s): %v", key, err)
	}

	for _, cond := range dep.Status.Conditions {
		if cond.Type == successfulDeployment && cond.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	return false, fmt.Errorf("deployment (%s) does not meet condition: %s", key, successfulDeployment)
}

func (a *aggregator) service(ctx context.Context, key client.ObjectKey, c addoncluster.Cluster) (bool, error) {
	svc := &corev1.Service{}
	err := c.GetClient().Get(ctx, key, svc)
	if err != nil {
		return false, fmt.Errorf("error reading service (%s): %v", key, err)
	}

	return true, nil
}
