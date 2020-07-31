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

package addon

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// Application Constants
const (
	// Used to indicate that not all of application's components
	// have been deployed yet.
	Pending = "Pending"
	// Used to indicate that all of application's components
	// have already been deployed.
	Succeeded = "Succeeded"
	// Used to indicate that deployment of application's components
	// failed. Some components might be present, but deployment of
	// the remaining ones will not be re-attempted.
	Failed = "Failed"
)

// TransformApplicationFromStatus modifies the Application in the deployment based off the CommonStatus
func TransformApplicationFromStatus(ctx context.Context, instance declarative.DeclarativeObject, objects *manifest.Objects) error {
	var version string
	var healthy bool
	var addonObject addonsv1alpha1.CommonObject

	if unstruct, ok := instance.(*unstructured.Unstructured); ok {
		v, _, err := unstructured.NestedString(unstruct.Object, "spec", "version")
		if err != nil {
			return fmt.Errorf("unable to get version from unstuctured: %v", err)
		}
		version = v

		healthy, _, err = unstructured.NestedBool(unstruct.Object, "status", "healthy")
		if err != nil {
			return fmt.Errorf("unable to get status from unstuctured: %v", err)
		}
	} else if addonObject, ok = instance.(addonsv1alpha1.CommonObject); ok {
		version = addonObject.CommonSpec().Version
		healthy = addonObject.GetCommonStatus().Healthy
	} else {
		return fmt.Errorf("instance %T was not an addonsv1alpha1.CommonObject", instance)
	}

	app, err := declarative.ExtractApplication(objects)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("cannot TransformApplicationFromStatus without an app.k8s.io/Application in the manifest")
	}

	assemblyPhase := Pending
	if healthy {
		assemblyPhase = Succeeded
	}

	// TODO: Version should be on CommonStatus as well
	app.SetNestedField(version, "spec", "descriptor", "version")
	app.SetNestedField(assemblyPhase, "spec", "assemblyPhase")

	return nil
}
