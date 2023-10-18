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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

// CRDHook implements functionality for CRD objects (the definitions themselves, not instances of CRDs)
type CRDHook struct {
	storage storage.Storage
}

func (s *CRDHook) OnWatchEvent(ev *storage.WatchEvent) {
	switch ev.GroupKind() {
	case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
		// When a CRD is created, we notify the storage layer so it can store instances of the CRD
		if err := s.storage.UpdateCRD(ev); err != nil {
			klog.Warningf("crd change was invalid: %v", err)
		}
	}
}
