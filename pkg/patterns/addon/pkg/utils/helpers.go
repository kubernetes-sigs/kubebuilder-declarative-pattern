package utils

import (
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

func genError(v runtime.Object) error {
	return fmt.Errorf("instance %T is not addonsv1alpha1.CommonObject or unstructured", v)
}

func GetCommonStatus(instance runtime.Object) (addonsv1alpha1.CommonStatus, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.GetCommonStatus(), nil
	case *unstructured.Unstructured:
		unstructStatus, _, err := unstructured.NestedMap(v.Object, "status")
		if err != nil {
			return addonsv1alpha1.CommonStatus{}, fmt.Errorf("unable to get status from unstuctured: %v", err)
		}
		var addonStatus addonsv1alpha1.CommonStatus
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructStatus, &addonStatus)
		if err != nil {
			return addonStatus, err
		}

		return addonStatus, nil
	default:
		return addonsv1alpha1.CommonStatus{}, genError(v)
	}
}

func SetCommonStatus(instance runtime.Object, status addonsv1alpha1.CommonStatus) error {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		v.SetCommonStatus(status)
	case *unstructured.Unstructured:
		unstructStatus, err := runtime.DefaultUnstructuredConverter.ToUnstructured(status)
		if err != nil {
			return fmt.Errorf("unable to convert unstructured to addonStatus: %v", err)
		}

		err = unstructured.SetNestedMap(v.Object, unstructStatus, "status")
		if err != nil {
			return fmt.Errorf("unable to set status in unstructured: %v", err)
		}

		return nil
	default:
		return genError(v)
	}
	return nil
}

func GetPatchableConditions(instance runtime.Object) (addonsv1alpha1.PatchableConditions, error) {
	if v, ok := instance.(addonsv1alpha1.ConditionGetterSetter); ok {
		return v.GetConditions(), nil
	}
	// instance cannot type assert to *unstructured.Unstructured directly.
	v, err := runtime.DefaultUnstructuredConverter.ToUnstructured(instance)
	if err != nil {
		return addonsv1alpha1.PatchableConditions{}, err
	}
	unstructConditions, _, err := unstructured.NestedSlice(v, "status", "conditions")
	if err != nil {
		return addonsv1alpha1.PatchableConditions{}, fmt.Errorf("unable to get status.conditions from unstuctured: %v", err)
	}
	var addonConditions addonsv1alpha1.PatchableConditions
	for _, ucond := range unstructConditions {
		var addonCond v1.Condition
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(ucond.(map[string]interface{}), &addonCond)
		if err != nil {
			return addonConditions, err
		}
		addonConditions.Conditions = append(addonConditions.Conditions, addonCond)
	}
	return addonConditions, nil
}

type MissingConditionsErr struct {
	Object runtime.Object
}

func (e *MissingConditionsErr) Error() string {
	return fmt.Sprintf("unable to find `status.condition` in %T", e.Object)
}

func SetPatchableConditions(instance runtime.Object, conditions addonsv1alpha1.PatchableConditions) error {
	if v, ok := instance.(addonsv1alpha1.ConditionGetterSetter); ok {
		v.SetConditions(conditions)
		return nil
	}
	newConditionsVal := reflect.ValueOf(conditions).FieldByName("Conditions")
	statusVal := reflect.ValueOf(instance).Elem().FieldByName("Status")
	if !statusVal.IsValid() {
		// Status not ready.
		return nil
	}
	conditionsVal := statusVal.FieldByName("Conditions")
	if !conditionsVal.IsValid() {
		return &MissingConditionsErr{Object: instance}
	}
	conditionsVal.Set(newConditionsVal)
	return nil
}

func GetCommonSpec(instance runtime.Object) (addonsv1alpha1.CommonSpec, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.CommonSpec(), nil
	case *unstructured.Unstructured:
		unstructSpec, _, err := unstructured.NestedMap(v.Object, "spec")
		if err != nil {
			return addonsv1alpha1.CommonSpec{}, fmt.Errorf("unable to get spec from unstuctured: %v", err)
		}
		var addonSpec addonsv1alpha1.CommonSpec
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructSpec, &addonSpec)
		if err != nil {
			return addonSpec, err
		}

		return addonSpec, nil
	default:
		return addonsv1alpha1.CommonSpec{}, genError(v)
	}
}

func GetCommonName(instance runtime.Object) (string, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.ComponentName(), nil
	case *unstructured.Unstructured:
		return strings.ToLower(v.GetKind()), nil
	default:
		return "", genError(v)
	}
}
