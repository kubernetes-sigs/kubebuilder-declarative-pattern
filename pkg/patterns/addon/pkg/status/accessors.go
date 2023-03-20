package status

import (
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetConditions(instance runtime.Object) ([]metav1.Condition, error) {
	statusVal := reflect.ValueOf(instance).Elem().FieldByName("Status")
	if !statusVal.IsValid() {
		return nil, fmt.Errorf("status field not found")
	}
	conditionsVal := statusVal.FieldByName("Conditions")
	if !conditionsVal.IsValid() {
		return nil, fmt.Errorf("status.conditions field not found")
		//  &MissingConditionsErr{Object: instance}
	}

	v := conditionsVal.Interface()
	conditions, ok := v.([]metav1.Condition)
	if !ok {
		return nil, fmt.Errorf("unexpecetd type for status.conditions; got %T, want []metav1.Condition", v)
	}
	return conditions, nil

	// if v, ok := instance.(addonsv1alpha1.ConditionGetterSetter); ok {
	// 	return v.GetConditions(), nil
	// }
	// instance cannot type assert to *unstructured.Unstructured directly.
	// v, err := runtime.DefaultUnstructuredConverter.ToUnstructured(instance)
	// if err != nil {
	// 	return nil, err
	// }
	// unstructConditions, _, err := unstructured.NestedSlice(v, "status", "conditions")
	// if err != nil {
	// 	return nil, fmt.Errorf("unable to get status.conditions from unstuctured: %v", err)
	// }
	// var conditions []metav1.Condition
	// for _, ucond := range unstructConditions {
	// 	var addonCond metav1.Condition
	// 	err = runtime.DefaultUnstructuredConverter.FromUnstructured(ucond.(map[string]interface{}), &addonCond)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	conditions = append(conditions, addonCond)
	// }
	// return conditions, nil
}

// type MissingConditionsErr struct {
// 	Object runtime.Object
// }

// func (e *MissingConditionsErr) Error() string {
// 	return fmt.Sprintf("unable to find `status.condition` in %T", e.Object)
// }

func SetConditions(instance runtime.Object, newConditions []metav1.Condition) error {
	// if v, ok := instance.(addonsv1alpha1.ConditionGetterSetter); ok {
	// 	v.SetConditions(conditions)
	// 	return nil
	// }
	// newConditionsVal := reflect.ValueOf(conditions).FieldByName("Conditions")
	statusVal := reflect.ValueOf(instance).Elem().FieldByName("Status")
	if !statusVal.IsValid() {
		// Status not ready.
		return fmt.Errorf("status field not found")
	}
	conditionsVal := statusVal.FieldByName("Conditions")
	if !conditionsVal.IsValid() {
		return fmt.Errorf("status.conditions field not found")
		//  &MissingConditionsErr{Object: instance}
	}

	newConditionsVal := reflect.ValueOf(newConditions)
	if !conditionsVal.CanSet() {
		return fmt.Errorf("cannot set status.conditions field")
	}
	if !newConditionsVal.CanConvert(conditionsVal.Type()) {
		return fmt.Errorf("cannot set type %v to status.conditions type %v", newConditionsVal.Type(), conditionsVal.Type())
	}
	conditionsVal.Set(newConditionsVal)

	return nil
}
