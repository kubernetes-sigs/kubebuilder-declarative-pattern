package mockkubeapiserver

import (
	"fmt"

	"k8s.io/klog/v2"
)

// typeInfo holds schema information used for schema-aware merging
type typeInfo struct {
	Name       string
	Properties map[string]propertyInfo
}

// propertyInfo holds info about a specified property
type propertyInfo struct {
	Key           string
	Type          *typeInfo
	PatchStrategy string
	MergeKey      string
}

// applyPatch applies the patch to the given object, based on type information in objectType
func applyPatch(existing, patch map[string]interface{}, objectType typeInfo) error {
	for k, patchValue := range patch {
		property := objectType.Properties[k]

		existingValue := existing[k]
		switch patchValue := patchValue.(type) {
		case string, int64, bool:
			existing[k] = patchValue

		case map[string]interface{}:
			if existingValue == nil {
				existing[k] = patchValue
			} else {
				existingMap, ok := existingValue.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected type mismatch, expected map got %T", existingValue)
				}
				var propertyType typeInfo
				if property.Type == nil {
					klog.Warningf("type of property is not known for %s/%s", objectType.Name, k)
				} else {
					propertyType = *property.Type
				}
				if err := applyPatch(existingMap, patchValue, propertyType); err != nil {
					return err
				}
			}

		case []interface{}:
			if property.MergeKey == "" {
				return fmt.Errorf("merge-key not known for %s/%s", objectType.Name, k)
			}

			if existingValue == nil {
				existing[k] = patchValue
			} else {
				existingSlice, ok := existingValue.([]interface{})
				if !ok {
					return fmt.Errorf("unexpected type mismatch, expected slice got %T", existingValue)
				}
				if err := applyPatchToList(existingSlice, patchValue, property); err != nil {
					return err
				}
			}

		default:
			return fmt.Errorf("type %T not handled in patch", patchValue)
		}
	}
	return nil
}

// applyPatchToList patches a slice, based on the strategy defined in propertyInfo
// List merging is based around the idea of a merge-key; these are really more like maps
func applyPatchToList(existingSlice []interface{}, patchSlice []interface{}, property propertyInfo) error {
	var propertyType typeInfo
	if property.Type == nil {
		klog.Warningf("type of property is not known for %s", property.Key)
	} else {
		propertyType = *property.Type
	}

	existingByKey := make(map[string]interface{})
	for _, obj := range existingSlice {
		key, err := extractKey(obj, property)
		if err != nil {
			return err
		}
		existingByKey[key] = obj
	}

	for _, patch := range patchSlice {
		key, err := extractKey(patch, property)
		if err != nil {
			return err
		}
		existing, exists := existingByKey[key]
		if !exists {
			existingSlice = append(existingSlice, patch)
			continue
		}

		if err := applyPatch(existing.(map[string]interface{}), patch.(map[string]interface{}), propertyType); err != nil {
			return err
		}
	}
	return nil
}

// extractKey gets the merge-key from the given object
func extractKey(obj interface{}, property propertyInfo) (string, error) {
	if property.MergeKey == "" {
		return "", fmt.Errorf("merge key not set in %q", property.Key)
	}
	switch obj := obj.(type) {
	case map[string]interface{}:
		name, found := obj[property.MergeKey]
		if !found {
			return "", fmt.Errorf("name not found in object in patch")
		}
		return name.(string), nil
	default:
		return "", fmt.Errorf("type %T not handled in extractKeys", obj)
	}
}
