package mockkubeapiserver

import "k8s.io/apimachinery/pkg/runtime/schema"

// buildTypeInfo returns the type information for the given GVK
// Currently this is hard-coded, but we can evolve this to load CRDs / openapi schema if needed.
// We only actually need a very small portion of the OpenAPI information at the moment - just the merge keys.
func buildTypeInfo(gvk schema.GroupVersionKind) typeInfo {
	var i typeInfo
	i.Name = gvk.Group + "." + gvk.Kind
	i.Properties = map[string]propertyInfo{}

	// TODO: Make objectMetaTypeInfo shared if there are no absolute paths in it
	var objectMetaTypeInfo typeInfo
	objectMetaTypeInfo.Name = "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta"
	objectMetaTypeInfo.Properties = map[string]propertyInfo{}
	objectMetaTypeInfo.Properties["ownerReferences"] = propertyInfo{
		Key:           "ownerReferences",
		MergeKey:      "uid",
		PatchStrategy: "merge",
	}

	i.Properties["metadata"] = propertyInfo{Key: "metadata", Type: &objectMetaTypeInfo}

	return i
}
