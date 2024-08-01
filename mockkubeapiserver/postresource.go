/*
Copyright 2022 The Kubernetes Authors.

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

package mockkubeapiserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// postResource is a request to create a new resource
type postResource struct {
	baseRequest

	Group    string
	Version  string
	Resource string

	Namespace string
}

// Run serves the http request
func (req *postResource) Run(ctx context.Context, s *MockKubeAPIServer) error {
	gr := schema.GroupResource{Group: req.Group, Resource: req.Resource}

	resource := s.storage.FindResource(gr)
	if resource == nil {
		return req.writeErrorResponse(http.StatusNotFound)
	}

	bodyBytes, err := io.ReadAll(req.r.Body)
	if err != nil {
		return err
	}

	klog.V(4).Infof("post request %#v", string(bodyBytes))

	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(bodyBytes); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	id := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}

	if id.Namespace != req.Namespace {
		return fmt.Errorf("namespace in payload did not match namespace in URL")
	}
	if id.Name == "" {
		return fmt.Errorf("name must be provided in payload")
	}

	if resource.SetsGeneration() {
		obj.SetGeneration(1)
	}

	if err := beforeObjectCreation(ctx, obj); err != nil {
		return err
	}
	obj, err = s.admissionHooks.BeforeCreate(ctx, resource, obj)
	if err != nil {
		return fmt.Errorf("error from before-create hooks: %w", err)
	}

	if err := resource.CreateObject(ctx, id, obj); err != nil {
		return err
	}
	return req.writeResponse(obj)
}

var secretGVK = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "Secret",
}

func beforeObjectCreation(ctx context.Context, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	if gvk == secretGVK {
		return beforeSecretCreation(ctx, obj)
	}
	return nil
}

func beforeSecretCreation(ctx context.Context, obj *unstructured.Unstructured) error {
	// If there is any stringData, merge it into data
	stringData, _, err := unstructured.NestedStringMap(obj.Object, "stringData")
	if err != nil {
		return fmt.Errorf("getting Secret stringData: %w", err)
	}
	if len(stringData) == 0 {
		return nil
	}

	// Get a copy of data
	data, _, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil {
		return fmt.Errorf("getting Secret data: %w", err)
	}
	if data == nil {
		data = make(map[string]string)
	}
	for k, v := range stringData {
		data[k] = base64.StdEncoding.EncodeToString([]byte(v))
	}
	if err := unstructured.SetNestedStringMap(obj.Object, data, "data"); err != nil {
		return fmt.Errorf("setting Secret data: %w", err)
	}
	unstructured.RemoveNestedField(obj.Object, "stringData")

	return nil
}
