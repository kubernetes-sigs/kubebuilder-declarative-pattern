/*
Copyright 2023 The Kubernetes Authors.

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
// This package provides Parent object related methods.
package applyset

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Parent is aimed for adaption. We want users to us Parent rather than the third-party/forked ParentRef directly.
// This gives us more flexibility to change the third-party/forked without introducing breaking changes to users.
type Parent interface {
	GroupVersionKind() schema.GroupVersionKind
	Name() string
	Namespace() string
	RESTMapping() *meta.RESTMapping
	GetSubject() runtime.Object
}

// NewParentRef initialize a ParentRef object.
func NewParentRef(object runtime.Object, name, namespace string, rest *meta.RESTMapping) *ParentRef {
	return &ParentRef{
		object:      object,
		name:        name,
		namespace:   namespace,
		restMapping: rest,
	}
}

var _ Parent = &ParentRef{}

// ParentRef defines the Parent object information
type ParentRef struct {
	namespace   string
	name        string
	restMapping *meta.RESTMapping
	object      runtime.Object
}

// GroupVersionKind returns the parent GroupVersionKind
func (p *ParentRef) GroupVersionKind() schema.GroupVersionKind {
	return p.object.GetObjectKind().GroupVersionKind()
}

// Name returns the parent Name
func (p *ParentRef) Name() string {
	return p.name
}

// Namespace returns the parent Namespace
func (p *ParentRef) Namespace() string {
	return p.namespace
}

// RESTMapping returns the parent RESTMapping
func (p *ParentRef) RESTMapping() *meta.RESTMapping {
	return p.restMapping
}

// GetSubject returns the parent runtime.Object
func (p *ParentRef) GetSubject() runtime.Object {
	return p.object
}
