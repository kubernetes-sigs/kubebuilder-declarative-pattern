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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MetricsServerSpec defines the desired state of MetricsServer
type MetricsServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	addonv1alpha1.CommonSpec `json:",inline"`
	addonv1alpha1.PatchSpec  `json:",inline"`
}

// MetricsServerStatus defines the observed state of MetricsServer
type MetricsServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	addonv1alpha1.CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// MetricsServer is the Schema for the metricsservers API
type MetricsServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricsServerSpec   `json:"spec,omitempty"`
	Status MetricsServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetricsServerList contains a list of MetricsServer
type MetricsServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsServer `json:"items"`
}

var _ addonv1alpha1.CommonObject = &MetricsServer{}
var _ addonv1alpha1.Patchable = &MetricsServer{}

func init() {
	SchemeBuilder.Register(&MetricsServer{}, &MetricsServerList{})
}

func (c *MetricsServer) ComponentName() string {
	return "metrics-server"
}

func (c *MetricsServer) CommonSpec() addonv1alpha1.CommonSpec {
	return c.Spec.CommonSpec
}

func (c *MetricsServer) GetCommonStatus() addonv1alpha1.CommonStatus {
	return c.Status.CommonStatus
}

func (c *MetricsServer) SetCommonStatus(s addonv1alpha1.CommonStatus) {
	c.Status.CommonStatus = s
}

func (c *MetricsServer) PatchSpec() addonv1alpha1.PatchSpec {
	return c.Spec.PatchSpec
}
