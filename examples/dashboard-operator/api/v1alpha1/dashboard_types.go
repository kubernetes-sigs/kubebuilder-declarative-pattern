/*

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

// DashboardSpec defines the desired state of Dashboard
type DashboardSpec struct {
	addonv1alpha1.CommonSpec `json:",inline"`
	addonv1alpha1.PatchSpec  `json:",inline"`
	addonv1alpha1.ConfigSpec `json:",inline"`
}

// DashboardStatus defines the observed state of Dashboard
type DashboardStatus struct {
	addonv1alpha1.CommonStatus `json:",inline"`
}

var _ addonv1alpha1.CommonObject = &Dashboard{}
var _ addonv1alpha1.Patchable = &Dashboard{}
var _ addonv1alpha1.ConfigMapGeneratorAble = &Dashboard{}

// +kubebuilder:object:root=true

// Dashboard is the Schema for the dashboards API
type Dashboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashboardSpec   `json:"spec,omitempty"`
	Status DashboardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DashboardList contains a list of Dashboard
type DashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dashboard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dashboard{}, &DashboardList{})
}

func (c *Dashboard) ComponentName() string {
	return "dashboard"
}

func (c *Dashboard) CommonSpec() addonv1alpha1.CommonSpec {
	return c.Spec.CommonSpec
}

func (c *Dashboard) GetCommonStatus() addonv1alpha1.CommonStatus {
	return c.Status.CommonStatus
}

func (c *Dashboard) SetCommonStatus(s addonv1alpha1.CommonStatus) {
	c.Status.CommonStatus = s
}

func (c *Dashboard) PatchSpec() addonv1alpha1.PatchSpec {
	return c.Spec.PatchSpec
}

func (c *Dashboard) ConfigSpec() addonv1alpha1.ConfigSpec {
	return c.Spec.ConfigSpec
}
