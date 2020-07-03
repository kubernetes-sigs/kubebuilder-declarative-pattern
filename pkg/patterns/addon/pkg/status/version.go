package status

import (
	"context"
	"fmt"
	"github.com/blang/semver"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"

	//"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// NewVersionCheck provides an implementation of declarative.Reconciled that
// checks the version of the operator if it is up to the version required by the manifest
func NewVersionCheck(client client.Client, version string) *versionCheck {
	return &versionCheck{client, version}
}

type versionCheck struct {
	client  client.Client
	version string
}

func (p *versionCheck) VersionCheck(
	ctx context.Context,
	src declarative.DeclarativeObject,
	objs *manifest.Objects,
) (bool, error) {
	log := log.Log
	zeroVersion := semver.Version{}
	var versionNeededStr string
	maxVersion := zeroVersion

	// Look for annotation from any resource with the max version
	for _, obj := range objs.Items {
		unstruct := obj.UnstructuredObject().Object
		metadata := unstruct["metadata"].(map[string]interface{})
		annotations, ok := metadata["annotations"].(map[string]interface{})
		if ok {
			versionNeededStr, _ = annotations["addons.k8s.io/operator-version"].(string)
			log.WithValues("version", versionNeededStr).Info("Got version, %v")

			versionActual, err := semver.Make(versionNeededStr)
			if err != nil {
				log.WithValues("version", versionNeededStr).Info("Unable to convert string to version, skipping this object")
				continue
			}

			if versionActual.GT(maxVersion) {
				maxVersion = versionActual
			}
		}
	}

	// TODO(somtochi): Do we want to return an error when the version is invalid or just skip and use the operator?
	operatorVersion, err := semver.Make(p.version)
	if err != nil {
		log.WithValues("version", p.version).Info("Unable to convert string to version, skipping check")
		return true, nil
	}

	if maxVersion.Equals(zeroVersion) || !maxVersion.GT(operatorVersion) {
		return true, nil
	}

	addonObject, ok := src.(addonsv1alpha1.CommonObject)
	if !ok {
		return false, fmt.Errorf("object %T was not an addonsv1alpha1.CommonObject", src)
	}

	status := addonsv1alpha1.CommonStatus{
		Healthy: false,
		Errors: []string{
			fmt.Sprintf("Addons needs version %v, this operator is version %v", maxVersion.String(), operatorVersion.String()),
		},
	}
	log.WithValues("name", addonObject.GetName()).WithValues("status", status).Info("updating status")
	addonObject.SetCommonStatus(status)

	return false, fmt.Errorf("operator not qualified, manifest needs operator >= %v", maxVersion.String())
}
