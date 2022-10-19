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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// patchResource is a request to patch a single resource
type patchResource struct {
	resourceRequestBase
}

// Run serves the http request
func (req *patchResource) Run(ctx context.Context, s *MockKubeAPIServer) error {
	gr := schema.GroupResource{Group: req.Group, Resource: req.Resource}
	resource := s.storage.FindResource(gr)
	if resource == nil {
		return req.writeErrorResponse(http.StatusNotFound)
	}

	id := types.NamespacedName{Namespace: req.Namespace, Name: req.Name}
	existingObj, found, err := s.storage.GetObject(ctx, resource, id)
	if err != nil {
		return err
	}
	if !found {
		existingObj = nil
	}

	bodyBytes, err := ioutil.ReadAll(req.r.Body)
	if err != nil {
		return err
	}

	body := &unstructured.Unstructured{}
	// Can't use the MarshalJSON overload, it doesn't like missing kind etc
	if err := json.Unmarshal(bodyBytes, &body.Object); err != nil {
		return fmt.Errorf("failed to parse PATCH payload: %w", err)
	}

	// TODO: We need to implement patch properly
	klog.Infof("patch request %#v", string(bodyBytes))

	if !found {
		// TODO: Only if server-side-apply

		if req.SubResource != "" {
			// TODO: Is this correct for server-side-apply?
			return req.writeErrorResponse(http.StatusNotFound)
		}

		patched := body
		if err := s.storage.CreateObject(ctx, resource, id, patched); err != nil {
			return err
		}

		return req.writeResponse(patched)
	}

	original := existingObj.DeepCopy()

	if req.SubResource == "" {
		if err := applyPatch(existingObj.Object, body.Object, resource.TypeInfo); err != nil {
			klog.Warningf("error from patch: %v", err)
			return err
		}
	} else {
		// TODO: We need to implement put properly
		return fmt.Errorf("unknown subresource %q", req.SubResource)
	}

	// We dont' want to change the resourceVersion (and trigger watches) when the change is a no-op
	if reflect.DeepEqual(original, existingObj) {
		klog.Infof("patch did not change object")
		return req.writeResponse(original)
	}

	if err := s.storage.UpdateObject(ctx, resource, id, existingObj); err != nil {
		return err
	}
	return req.writeResponse(existingObj)
}
