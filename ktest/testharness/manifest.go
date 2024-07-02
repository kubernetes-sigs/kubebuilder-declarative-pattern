/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testharness

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

func ParseObjects(ctx context.Context, manifest string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured
	reader := k8syaml.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	for {
		raw, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return objects, nil
			}

			return nil, fmt.Errorf("reading YAML doc: %w", err)
		}

		u := &unstructured.Unstructured{}
		if err := k8syaml.Unmarshal(raw, &u); err != nil {
			return nil, fmt.Errorf("parsing object to unstructured: %w", err)
		}

		objects = append(objects, u)
	}

	return objects, nil
}
