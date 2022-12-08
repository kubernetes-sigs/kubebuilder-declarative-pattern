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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
)

// NewBasic provides an implementation of declarative.Status that
// performs no preflight checks.
//
// Deprecated: This function exists for backward compatibility, please use NewKstatusCheck
func NewBasic(client client.Client) declarative.Status {
	return &declarative.StatusBuilder{
		BuildStatusImpl: NewAggregator(client),
		// no preflight checks
	}
}

// NewBasicVersionCheck provides an implementation of declarative.Status that
// performs version checks for the version of the operator that the manifest requires.
func NewBasicVersionChecks(client client.Client, version string) (declarative.Status, error) {
	v, err := NewVersionCheck(client, version)
	if err != nil {
		return nil, err
	}

	return &declarative.StatusBuilder{
		BuildStatusImpl:  NewAggregator(client),
		VersionCheckImpl: v,
		// no preflight checks
	}, nil
}

// TODO: Create a version that doesn't take (unusued) client & reconciler args
func NewKstatusCheck(client client.Client, d *declarative.Reconciler) declarative.Status {
	return &declarative.StatusBuilder{
		BuildStatusImpl: NewKstatusAgregator(client, d),
	}
}
