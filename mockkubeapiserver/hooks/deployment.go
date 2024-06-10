/*
Copyright 2022 The Kubernetes Authors.

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

package hooks

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

type DeploymentHook struct {
}

func (s *DeploymentHook) OnWatchEvent(ev *storage.WatchEvent) {
	switch ev.GroupKind() {
	case schema.GroupKind{Group: "apps", Kind: "Deployment"}:
		if err := s.deploymentChanged(ev); err != nil {
			klog.Fatalf("could not update deployment status: %v", err)
		}
	}
}

// Fake the required transitions for Deployment objects so they become ready
func (s *DeploymentHook) deploymentChanged(ev *storage.WatchEvent) error {
	u := ev.Unstructured()

	// So that deployments become ready, we immediately update the status.
	// We could do something better here, like e.g. a 1 second pause before changing the status
	statusObj := u.Object["status"]
	if statusObj == nil {
		statusObj = make(map[string]interface{})
		u.Object["status"] = statusObj
	}
	status, ok := statusObj.(map[string]interface{})
	if !ok {
		return fmt.Errorf("status was of unexpected type %T", statusObj)
	}

	replicasVal, _, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "replicas")
	if err != nil {
		return fmt.Errorf("error getting spec.replicas: %w", err)
	}
	replicas := int64(0)
	switch replicasVal := replicasVal.(type) {
	case int64:
		replicas = replicasVal
	case float64:
		replicas = int64(replicasVal)
	default:
		return fmt.Errorf("unhandled type for spec.replicas %T", replicasVal)
	}

	var conditions []interface{}
	conditions = append(conditions, map[string]interface{}{
		"type":   "Available",
		"status": "True",
		"reason": "MinimumReplicasAvailable",
	})
	conditions = append(conditions, map[string]interface{}{
		"type":   "Progressing",
		"status": "True",
		"reason": "NewReplicaSetAvailable",
	})
	status["conditions"] = conditions

	status["availableReplicas"] = replicas
	status["readyReplicas"] = replicas
	status["replicas"] = replicas
	status["updatedReplicas"] = replicas

	observedGeneration := u.GetGeneration()
	status["observedGeneration"] = observedGeneration

	return nil
}
