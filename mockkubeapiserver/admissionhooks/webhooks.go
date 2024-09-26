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

package admissionhooks

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	admissionv1 "sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/internal/api/admission/v1"
	admissionregistrationv1 "sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/internal/api/admissionregistration/v1"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

// Webhooks manages our kubernetes admission webhooks (both validating and mutating)
type Webhooks struct {
	// TODO: Replace with a copy-on-write mechanism
	mutex          sync.Mutex
	mutatingByName map[string]*mutatingWebhookRecord
}

// New constructs an instance of Webhooks.
func New() *Webhooks {
	h := &Webhooks{}
	h.mutatingByName = make(map[string]*mutatingWebhookRecord)
	return h
}

// OnWatchEvent is called by the storage system for any change.
// We observe changes to webhook objects and set up webhooks
func (s *Webhooks) OnWatchEvent(ev *storage.WatchEvent) {
	switch ev.GroupKind() {
	case schema.GroupKind{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"}:

		// TODO: Deleted / changed webhooks

		u := ev.Unstructured()

		webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, webhook); err != nil {
			klog.Fatalf("failed to parse webhook: %v", err)
		}

		if err := s.update(webhook); err != nil {
			klog.Fatalf("failed to update webhook: %v", err)
		}
	}
}

// update is called when a mutating webhook changes; we record the webhook details.
func (w *Webhooks) update(obj *admissionregistrationv1.MutatingWebhookConfiguration) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	name := obj.GetName()
	existing := w.mutatingByName[name]
	if existing != nil {
		existing.obj = obj
	} else {
		existing = &mutatingWebhookRecord{obj: obj}
		w.mutatingByName[name] = existing
	}

	existing.webhooks = make([]*mutatingWebhook, 0, len(obj.Webhooks))
	for i := range obj.Webhooks {
		webhookObj := &obj.Webhooks[i]
		existing.webhooks = append(existing.webhooks, &mutatingWebhook{webhook: webhookObj})
	}

	return nil
}

// BeforeCreate should be invoked before any object is created.
// We will invoke validating and mutating webhooks on the object.
func (w *Webhooks) BeforeCreate(ctx context.Context, resource storage.ResourceInfo, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	subresource := ""
	gvr := resource.GVR()

	matchingWebhooks, err := w.findMatchingWebhooks(admissionregistrationv1.Create, gvr, subresource)
	if err != nil {
		return nil, err
	}
	if len(matchingWebhooks) == 0 {
		return obj, nil
	}

	req := newAdmissionRequest(obj, admissionv1.Create, gvr)
	updated, err := w.invoke(ctx, matchingWebhooks, req, obj)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		obj = updated
		// TODO: Looping if object changes
	}
	return obj, nil
}

// findMatchingWebhooks returns the webhooks that we need to call.
func (w *Webhooks) findMatchingWebhooks(operation admissionregistrationv1.OperationType, gvr schema.GroupVersionResource, subresource string) ([]*mutatingWebhook, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	var allMatches []*mutatingWebhook
	for _, webhookSet := range w.mutatingByName {
		for _, webhook := range webhookSet.webhooks {
			isMatch, err := webhook.isMatch(operation, gvr, subresource)
			if err != nil {
				return nil, err
			}
			if isMatch {
				allMatches = append(allMatches, webhook)
			}
		}
	}
	return allMatches, nil
}

// invoke makes the webhook requests to a chain of webhooks.
func (w *Webhooks) invoke(ctx context.Context, matches []*mutatingWebhook, req *admissionRequest, original *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	for _, match := range matches {
		updated, err := match.invoke(ctx, req, original)
		if err != nil {
			return nil, err
		}
		if updated != nil {
			return updated, nil
		}
	}
	return nil, nil
}

// mutatingWebhookRecord is our tracking data structure for a mutatingWebhook
type mutatingWebhookRecord struct {
	obj      *admissionregistrationv1.MutatingWebhookConfiguration
	webhooks []*mutatingWebhook
}

type mutatingWebhook struct {
	webhook *admissionregistrationv1.MutatingWebhook
}

func (w *mutatingWebhook) isMatch(operation admissionregistrationv1.OperationType, gvr schema.GroupVersionResource, subresource string) (bool, error) {
	webhook := w.webhook
	if webhook.NamespaceSelector != nil {
		return false, fmt.Errorf("webhook namespaceSelector not implemented")
	}
	if webhook.ObjectSelector != nil {
		return false, fmt.Errorf("webhook objectSelector not implemented")
	}
	if webhook.MatchPolicy != nil {
		return false, fmt.Errorf("webhook matchPolicy not implemented")
	}
	for _, rule := range webhook.Rules {
		if rule.Scope != nil {
			return false, fmt.Errorf("webhook scope not implemented")
		}

		matchOperations := false
		for _, op := range rule.Operations {
			if op == "*" {
				matchOperations = true
			} else if op == operation {
				matchOperations = true
			}
		}
		if !matchOperations {
			continue
		}

		matchGroup := false
		for _, group := range rule.APIGroups {
			if group == "*" {
				matchGroup = true
			} else if group == gvr.Group {
				matchGroup = true
			}
		}
		if !matchGroup {
			continue
		}
		matchResource := false
		for _, resource := range rule.Resources {
			tokens := strings.Split(resource, "/")
			if len(tokens) == 1 {
				if resource == "" {
					// Empty-string ("") means "all resources, but not subresources"
					matchResource = subresource == ""
				} else if tokens[0] == gvr.Resource {
					matchResource = subresource == ""
				}
			} else if len(tokens) == 2 {
				if resource == "/*" {
					// `/*` means "all resources, and their subresources"
					matchResource = true
				} else if tokens[0] == gvr.Resource {
					if tokens[1] == "" || tokens[1] == gvr.Resource {
						matchResource = true
					}
				} else if tokens[0] == "" {
					if tokens[1] == subresource {
						matchResource = true
					}
				}
			}
		}
		if !matchResource {
			continue
		}

		return true, nil
	}

	return false, nil
}

// admissionRequest holds the data for an admission webhook call.
type admissionRequest struct {
	req *admissionv1.AdmissionReview
}

// newAdmissionRequest constructs an admissionRequest object.
func newAdmissionRequest(obj *unstructured.Unstructured, op admissionv1.Operation, gvr schema.GroupVersionResource) *admissionRequest {
	gvk := obj.GroupVersionKind()

	req := &admissionv1.AdmissionReview{}
	req.APIVersion = "admission.k8s.io/v1"
	req.Kind = "AdmissionReview"
	req.Request = &admissionv1.AdmissionRequest{}
	req.Request.Kind = metav1.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	}
	req.Request.Resource = metav1.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: gvr.Resource,
	}
	req.Request.Name = obj.GetName()
	req.Request.Namespace = obj.GetNamespace()
	req.Request.Operation = op
	req.Request.Object = runtime.RawExtension{Object: obj}

	r := &admissionRequest{req: req}

	return r
}

func (r *admissionRequest) requestJSON() ([]byte, error) {
	body, err := json.Marshal(r.req)
	if err != nil {
		return nil, fmt.Errorf("building webhook request: %w", err)
	}
	return body, nil
}

// invoke makes the webhook request to a specific webhook.
func (c *mutatingWebhook) invoke(ctx context.Context, req *admissionRequest, u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	clientConfig := c.webhook.ClientConfig

	tlsConfig := &tls.Config{}
	if len(clientConfig.CABundle) != 0 {
		caBundle := x509.NewCertPool()
		if ok := caBundle.AppendCertsFromPEM(clientConfig.CABundle); !ok {
			return nil, fmt.Errorf("no CA certificates found in caBundle")
		}
		tlsConfig.RootCAs = caBundle
	}

	url := ""
	if clientConfig.URL != nil {
		url = *clientConfig.URL
	}
	if clientConfig.Service != nil {
		return nil, fmt.Errorf("webhook clientConfig.Service not implemented")
	}
	if url == "" {
		return nil, fmt.Errorf("cannot determine URL for webhook")
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	httpRequestBody, err := req.requestJSON()
	if err != nil {
		return nil, fmt.Errorf("building webhook request: %w", err)
	}
	klog.Infof("sending webhook request: %v", string(httpRequestBody))

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(httpRequestBody))
	if err != nil {
		return nil, fmt.Errorf("building http request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("calling webhook: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webhook returned unexpected status %q", httpResponse.Status)
	}

	httpResponseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("reading webhook response body: %w", err)
	}

	admissionResponse := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(httpResponseBody, &admissionResponse); err != nil {
		return nil, fmt.Errorf("parsing webhook response: %w", err)
	}
	if admissionResponse.Response == nil {
		return nil, fmt.Errorf("webhook response is nil")
	}
	klog.Infof("admission response: %v", string(httpResponseBody))
	if !admissionResponse.Response.Allowed {
		return nil, fmt.Errorf("webhook blocked request")
	}

	if admissionResponse.Response.Patch != nil {
		if admissionResponse.Response.PatchType == nil || *admissionResponse.Response.PatchType != admissionv1.PatchTypeJSONPatch {
			return nil, fmt.Errorf("unhandled webhook patchType %q", *admissionResponse.Response.PatchType)
		}
		patch, err := jsonpatch.DecodePatch(admissionResponse.Response.Patch)
		if err != nil {
			return nil, fmt.Errorf("decoding webhook patch: %w", err)
		}
		beforePatch, err := json.Marshal(u)
		if err != nil {
			return nil, fmt.Errorf("building json for object: %w", err)
		}
		afterPatch, err := patch.Apply(beforePatch)
		if err != nil {
			return nil, fmt.Errorf("applying webhook patch: %w", err)
		}

		u2 := &unstructured.Unstructured{}
		if err := json.Unmarshal(afterPatch, u2); err != nil {
			return nil, fmt.Errorf("unmarshalling patched object: %w", err)
		}
		klog.Infof("after patch: %v", u2)
		return u2, nil
	}
	return nil, nil
}
