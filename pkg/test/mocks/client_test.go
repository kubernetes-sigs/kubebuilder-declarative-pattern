/*
Copyright 2018 The Kubernetes Authors.

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

package mocks

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestGetNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	ctx := context.Background()
	client := NewClient(scheme)

	node := v1.Node{}
	err := client.Get(ctx, types.NamespacedName{Name: "foo"}, &node)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected IsNotFound error from non-existent get, got %v", err)
	}
}

func TestCreateAndGet(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	ctx := context.Background()
	client := NewClient(scheme)

	node := v1.Node{}

	err := client.Get(ctx, types.NamespacedName{Name: "foo"}, &node)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected IsNotFound error from non-existent get, got %v", err)
	}

	node.Name = "foo"
	node.Spec.PodCIDR = "10.0.0.0/8"

	err = client.Create(ctx, &node)
	if err != nil {
		t.Fatalf("expected Create(node) to succeed; got error %v", err)
	}

	fetched := v1.Node{}
	err = client.Get(ctx, types.NamespacedName{Name: "foo"}, &fetched)
	if err != nil {
		t.Fatalf("expected Get(node) to succeed; got error %v", err)
	}

	if fetched.Spec.PodCIDR != "10.0.0.0/8" {
		t.Fatalf("expected node to round-trip, but PodCIDR was %q", fetched.Spec.PodCIDR)
	}
}
