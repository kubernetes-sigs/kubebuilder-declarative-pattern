package utils

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

func GetCommonVersion(instance runtime.Object) (string, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.CommonSpec().Version, nil
	case *unstructured.Unstructured:
		version, _, err := unstructured.NestedString(v.Object, "spec", "version")
		if err != nil {
			return "", fmt.Errorf("unable to get version from unstuctured: %v", err)
		}
		return version, nil
	default:
		return "", fmt.Errorf("instance %T is not addonsv1alpha1.CommonObject or unstructured", v)
	}
}

func GetCommonHealth(instance runtime.Object) (bool, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.GetCommonStatus().Healthy, nil
	case *unstructured.Unstructured:
		version, _, err := unstructured.NestedBool(v.Object, "status", "healthy")
		if err != nil {
			return false, fmt.Errorf("unable to get version from unstuctured: %v", err)
		}
		return version, nil
	default:
		return false, fmt.Errorf("instance %T is not addonsv1alpha1.CommonObject or unstructured", v)
	}
}

func GetCommonChannel(instance runtime.Object) (string, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.CommonSpec().Channel, nil
	case *unstructured.Unstructured:
		channel, _, err := unstructured.NestedString(v.Object, "spec", "channel")
		if err != nil {
			return "", fmt.Errorf("unable to get version from unstuctured: %v", err)
		}
		return channel, nil
	default:
		return "", fmt.Errorf("instance %T is not addonsv1alpha1.CommonObject or unstructured", v)
	}
}

func GetCommonName(instance runtime.Object) (string, error) {
	switch v := instance.(type) {
	case addonsv1alpha1.CommonObject:
		return v.ComponentName(), nil
	case *unstructured.Unstructured:
		return strings.ToLower(v.GetKind()), nil
	default:
		return "", fmt.Errorf("instance %T is not addonsv1alpha1.CommonObject or unstructured", v)
	}
}
