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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

type NamespaceHook struct {
}

func (s *NamespaceHook) OnWatchEvent(ev *storage.WatchEvent) {
	switch ev.GroupKind() {
	case schema.GroupKind{Kind: "Namespace"}:
		s.namespaceChanged(ev)
	}
}

// Fake the required transitions for Namespace objects so they become ready
func (s *NamespaceHook) namespaceChanged(ev *storage.WatchEvent) {
	u := ev.Unstructured()

	// These changes seem to be done synchronously (similar to a mutating webhook)
	labels := u.GetLabels()
	name := u.GetName()
	if labels["kubernetes.io/metadata.name"] != name {
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["kubernetes.io/metadata.name"] = name
		u.SetLabels(labels)
	}
	phase, _, _ := unstructured.NestedFieldNoCopy(u.Object, "status", "phase")
	if phase != "Active" {
		unstructured.SetNestedField(u.Object, "Active", "status", "phase")
	}
	found := false
	finalizers, _, _ := unstructured.NestedSlice(u.Object, "spec", "finalizers")
	for _, finalizer := range finalizers {
		if finalizer == "kubernetes" {
			found = true
		}
	}
	if !found {
		finalizers = append(finalizers, "kubernetes")
		unstructured.SetNestedSlice(u.Object, finalizers, "spec", "finalizers")
	}
}
