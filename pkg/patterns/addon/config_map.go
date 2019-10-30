// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0
package addon

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	addonsv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// ApplyConfigMapGenerator is an ObjectTransform to apply configMapGenerator specified on the Addon object to the manifest
// This transform requires the DeclarativeObject to implement addonsv1alpha1.ConfigMapGeneratorAble
func ApplyConfigMapGenerator(ctx context.Context, object declarative.DeclarativeObject, objects *manifest.Objects) error {
	log := log.Log

	config, ok := object.(addonsv1alpha1.ConfigMapGeneratorAble)
	if !ok {
		return fmt.Errorf("provided object (%T) does not implement ConfigMapGeneratorAble type", object)
	}

	cm_tmp := config.ConfigSpec().ConfigMapGenerator
	r := bytes.NewReader(cm_tmp.Raw)
	decoder := yaml.NewYAMLOrJSONDecoder(r, 1024)
	cm := &unstructured.Unstructured{}
	if err := decoder.Decode(cm); err != nil {
		return fmt.Errorf("error parsing json into unstructured object: %v", err)
	}
	log.WithValues("config map generator ", cm).V(1).Info("generated!!!")

	return objects.Hash(cm)
}
