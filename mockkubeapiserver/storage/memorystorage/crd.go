package memorystorage

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

// UpdateCRD is called whenever a CRD is updated.
// We register the types from the CRD.
func (s *MemoryStorage) UpdateCRD(ev *storage.WatchEvent) error {
	// TODO: Deleted / changed CRDs

	u := ev.Unstructured()

	group, _, _ := unstructured.NestedString(u.Object, "spec", "group")
	if group == "" {
		return fmt.Errorf("spec.group not set")
	}

	kind, _, _ := unstructured.NestedString(u.Object, "spec", "names", "kind")
	if kind == "" {
		return fmt.Errorf("spec.names.kind not set")
	}

	resource, _, _ := unstructured.NestedString(u.Object, "spec", "names", "plural")
	if resource == "" {
		return fmt.Errorf("spec.names.plural not set")
	}

	scope, _, _ := unstructured.NestedString(u.Object, "spec", "scope")
	if scope == "" {
		return fmt.Errorf("spec.scope not set")
	}

	versionsObj, found, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "versions")
	if !found {
		return fmt.Errorf("spec.versions not set")
	}

	versions, ok := versionsObj.([]interface{})
	if !ok {
		return fmt.Errorf("spec.versions not a slice")
	}

	for _, versionObj := range versions {
		version, ok := versionObj.(map[string]interface{})
		if !ok {
			return fmt.Errorf("spec.versions element not an object")
		}

		versionName, _, _ := unstructured.NestedString(version, "name")
		if versionName == "" {
			return fmt.Errorf("version name not set")
		}
		gvk := schema.GroupVersionKind{Group: group, Version: versionName, Kind: kind}
		gvr := gvk.GroupVersion().WithResource(resource)
		gr := gvr.GroupResource()

		storage := &resourceStorage{
			GroupResource:        gr,
			objects:              make(map[types.NamespacedName]*unstructured.Unstructured),
			parent:               s,
			resourceVersionClock: &s.resourceVersionClock,
		}

		// TODO: share storage across different versions
		s.resourceStorages[gr] = storage

		r := &memoryResourceInfo{
			api: metav1.APIResource{
				Name:    resource,
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			gvk:     gvk,
			gvr:     gvr,
			parent:  s,
			storage: storage,
		}
		r.listGVK = gvk.GroupVersion().WithKind(gvk.Kind + "List")

		// TODO: Set r.TypeInfo from schema

		switch scope {
		case "Namespaced":
			r.api.Namespaced = true
		case "Cluster":
			r.api.Namespaced = false
		default:
			return fmt.Errorf("scope %q is not recognized", scope)
		}

		s.schema.resources = append(s.schema.resources, r)
	}
	return nil
}
