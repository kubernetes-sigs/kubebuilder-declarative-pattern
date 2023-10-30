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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

// CRDHook implements functionality for CRD objects (the definitions themselves, not instances of CRDs)
type CRDHook struct {
	Storage storage.Storage
}

func (s *CRDHook) OnWatchEvent(ev *storage.WatchEvent) {
	switch ev.GroupKind() {
	case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
		// When a CRD is created, we notify the storage layer so it can store instances of the CRD
		if err := s.Storage.UpdateCRD(ev); err != nil {
			klog.Warningf("crd change was invalid: %v", err)
		}

		if err := s.updateCRDConditions(ev); err != nil {
			klog.Fatalf("could not update crd status: %v", err)
		}
	}
}
func (s *CRDHook) updateCRDConditions(ev *storage.WatchEvent) error {
	u := ev.Unstructured()

	// So that CRDs become ready, we immediately update the status.
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

	generation := u.GetGeneration()
	if generation == 0 {
		generation = 1
		u.SetGeneration(generation)
	}

	var conditions []interface{}
	conditions = append(conditions, map[string]interface{}{
		"type":    "NamesAccepted",
		"status":  "True",
		"reason":  "NoConflicts",
		"message": "no conflicts found",
	})
	conditions = append(conditions, map[string]interface{}{
		"type":    "Established",
		"status":  "True",
		"reason":  "InitialNamesAccepted",
		"message": "the initial names have been accepted",
	})
	status["conditions"] = conditions

	// TODO: More of status?  Here is an example of the full status
	// status:
	//	acceptedNames:
	//	  kind: VolumeSnapshot
	//	  listKind: VolumeSnapshotList
	//	  plural: volumesnapshots
	//	  singular: volumesnapshot
	//	conditions:
	//	- lastTransitionTime: "2023-09-21T01:04:36Z"
	//	  message: no conflicts found
	//	  reason: NoConflicts
	//	  status: "True"
	//	  type: NamesAccepted
	//	- lastTransitionTime: "2023-09-21T01:04:36Z"
	//	  message: the initial names have been accepted
	//	  reason: InitialNamesAccepted
	//	  status: "True"
	//	  type: Established
	//	- lastTransitionTime: "2023-09-21T01:04:36Z"
	//	  message: approved in https://github.com/kubernetes-csi/external-snapshotter/pull/419
	//	  reason: ApprovedAnnotation
	//	  status: "True"
	//	  type: KubernetesAPIApprovalPolicyConformant
	//	storedVersions:
	//	- v1

	return nil
}
