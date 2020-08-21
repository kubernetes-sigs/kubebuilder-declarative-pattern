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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
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

func (a *aggregator) Reconciled(ctx context.Context, src declarative.DeclarativeObject, objs *manifest.Objects) error {
	log := log.Log

	statusHealthy := true
	statusErrors := []string{}

	for _, o := range objs.Items {
		gk := o.Group + "/" + o.Kind
		healthy := true
		objKey := client.ObjectKey{
			Name:      o.Name,
			Namespace: o.Namespace,
		}
		// If the namespace isn't set on the object, we would want to use the namespace of src
		if objKey.Namespace == "" {
			objKey.Namespace = src.GetNamespace()
		}
		var err error
		switch gk {
		case "/Service":
			healthy, err = a.service(ctx, objKey)
		case "extensions/Deployment", "apps/Deployment":
			healthy, err = a.deployment(ctx, objKey)
		default:
			log.WithValues("type", gk).V(2).Info("type not implemented for status aggregation, skipping")
		}

		statusHealthy = statusHealthy && healthy
		if err != nil {
			statusErrors = append(statusErrors, fmt.Sprintf("%v", err))
		}
	}

	log.WithValues("object", src).WithValues("status", statusHealthy).V(2).Info("built status")

	unstruct, ok := src.(*unstructured.Unstructured)
	instance, commonOkay := src.(addonv1alpha1.CommonObject)
	changed := false

	if commonOkay {
		var status = instance.GetCommonStatus()
		status.Errors = statusErrors
		status.Healthy = statusHealthy

		if !reflect.DeepEqual(status, instance.GetCommonStatus()) {
			status.Healthy = statusHealthy

			log.WithValues("name", instance.GetName()).WithValues("status", status).Info("updating status")
			changed = true
		}
	} else if ok {
		unstructStatus := make(map[string]interface{})

		s, _, err := unstructured.NestedMap(unstruct.Object, "status")
		if err != nil {
			log.Error(err, "getting status")
			return fmt.Errorf("unable to get status from unstructured: %v", err)
		}

		unstructStatus = s
		unstructStatus["Healthy"] = statusHealthy
		unstructStatus["Errors"] = statusErrors
		if !reflect.DeepEqual(unstruct, s) {
			err = unstructured.SetNestedField(unstruct.Object, statusHealthy, "status", "healthy")
			if err != nil {
				log.Error(err, "updating status")
				return fmt.Errorf("unable to set status in unstructured: %v", err)
			}

			err = unstructured.SetNestedStringSlice(unstruct.Object, statusErrors, "status", "errors")
			if err != nil {
				log.Error(err, "updating status")
				return fmt.Errorf("unable to set status in unstructured: %v", err)
			}
			changed = true
		}
	} else {
		return fmt.Errorf("instance %T was not an addonsv1alpha1.CommonObject or unstructured.Unstructured", src)
	}

	if changed == true {
		log.WithValues("name", src.GetName()).WithValues("status", statusHealthy).Info("updating status")
		err := a.client.Status().Update(ctx, src)
		if err != nil {
			log.Error(err, "updating status")
			return err
		}
	}

	return nil
}

func (a *aggregator) deployment(ctx context.Context, key client.ObjectKey) (bool, error) {
	dep := &appsv1.Deployment{}

	if err := a.client.Get(ctx, key, dep); err != nil {
		return false, fmt.Errorf("error reading deployment (%s): %v", key, err)
	}

	for _, cond := range dep.Status.Conditions {
		if cond.Type == successfulDeployment && cond.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	return false, fmt.Errorf("deployment (%s) does not meet condition: %s", key, successfulDeployment)
}

func (a *aggregator) service(ctx context.Context, key client.ObjectKey) (bool, error) {
	svc := &corev1.Service{}
	err := a.client.Get(ctx, key, svc)
	if err != nil {
		return false, fmt.Errorf("error reading service (%s): %v", key, err)
	}

	return true, nil
}
