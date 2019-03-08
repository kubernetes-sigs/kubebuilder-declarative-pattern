package addon

import (
	"context"
	"errors"
	"fmt"

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
	addonObject, ok := instance.(addonsv1alpha1.CommonObject)
	if !ok {
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
	if addonObject.GetCommonStatus().Healthy {
		assemblyPhase = Succeeded
	}

	// TODO: Version should be on CommonStatus as well
	app.SetNestedField(addonObject.CommonSpec().Version, "spec", "descriptor", "version")
	app.SetNestedField(assemblyPhase, "spec", "assemblyPhase")

	return nil
}
