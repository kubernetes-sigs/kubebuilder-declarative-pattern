package declarative

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// AddLabels returns an ObjectTransform that adds labels to all the objects
func AddLabels(labels map[string]string) ObjectTransform {
	return func(ctx context.Context, o DeclarativeObject, manifest *manifest.Objects) error {
		// TODO: Add to selectors and labels in templates?
		for _, o := range manifest.Items {
			o.AddLabels(labels)
		}

		return nil
	}
}

// SourceLabel returns a fixed label based on the type and name of the DeclarativeObject
func SourceLabel(scheme *runtime.Scheme) LabelMaker {
	return func(ctx context.Context, o DeclarativeObject) map[string]string {
		log := log.Log

		gvk := o.GetObjectKind().GroupVersionKind()
		gvk, err := apiutil.GVKForObject(o, scheme)

		if err != nil {
			log.WithValues("object", o).WithValues("GroupVersionKind", gvk).Error(err, "can't map GroupVersionKind")
			return map[string]string{}
		}

		if gvk.Group == "" || gvk.Kind == "" {
			log.WithValues("object", o).WithValues("GroupVersionKind", gvk).Info("GroupVersionKind is invalid")
			return map[string]string{}
		}

		return map[string]string{
			fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind): o.GetName(),
		}
	}
}
