// application.go manages an Application[1]
//
// [1] https://github.com/kubernetes-sigs/application
package declarative

import (
	"context"
	"errors"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

func transformApplication(ctx context.Context, instance DeclarativeObject, objects *manifest.Objects, labelMaker LabelMaker) error {
	app, err := ExtractApplication(objects)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("cannot transformApplication without an app.k8s.io/Application in the manifest")
	}

	app.SetNestedFieldNoCopy(metav1.LabelSelector{MatchLabels: labelMaker(ctx, instance)}, "spec", "selector")
	app.SetNestedFieldNoCopy(uniqueGroupKind(objects), "spec", "componentGroupKinds")

	return nil
}

// uniqueGroupKind returns all unique GroupKind defined in objects
func uniqueGroupKind(objects *manifest.Objects) []metav1.GroupKind {
	kinds := map[metav1.GroupKind]struct{}{}
	for _, o := range objects.Items {
		gk := o.GroupKind()
		kinds[metav1.GroupKind{Group: gk.Group, Kind: gk.Kind}] = struct{}{}
	}
	var unique []metav1.GroupKind
	for gk := range kinds {
		unique = append(unique, gk)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].String() < unique[j].String()
	})
	return unique
}

// ExtractApplication extracts a single app.k8s.io/Application from objects.
//
// -  0 Application: (nil, nil)
// -  1 Application: (*app, nil)
// - >1 Application: (nil, err)
func ExtractApplication(objects *manifest.Objects) (*manifest.Object, error) {
	var app *manifest.Object
	for _, o := range objects.Items {
		if o.Group == "app.k8s.io" && o.Kind == "Application" {
			if app != nil {
				return nil, errors.New("multiple app.k8s.io/Application found in manifest")
			}
			app = o
		}
	}
	return app, nil
}
