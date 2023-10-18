package storage

import (
	"encoding/json"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

type WatchEvent struct {
	gvk            schema.GroupVersionKind
	internalObject *unstructured.Unstructured
	eventType      string

	Namespace string

	mutex                     sync.Mutex
	partialObjectMetadataJSON []byte
	json                      []byte
}

type messageV1 struct {
	Type   string         `json:"type"`
	Object runtime.Object `json:"object"`
}

func BuildWatchEvent(gvk schema.GroupVersionKind, evType string, u *unstructured.Unstructured) *WatchEvent {
	ev := &WatchEvent{
		gvk:            gvk,
		internalObject: u,
		eventType:      evType,
		Namespace:      u.GetNamespace(),
	}
	return ev
}

func (ev *WatchEvent) GroupKind() schema.GroupKind {
	return ev.gvk.GroupKind()
}

func (ev *WatchEvent) Unstructured() *unstructured.Unstructured {
	return ev.internalObject
}

func (ev *WatchEvent) JSON() []byte {
	ev.mutex.Lock()
	defer ev.mutex.Unlock()

	if ev.json != nil {
		return ev.json
	}
	u := ev.internalObject

	msg := messageV1{
		Type:   ev.eventType,
		Object: u,
	}

	j, err := json.Marshal(&msg)
	if err != nil {
		klog.Fatalf("error from json.Marshal(%T): %v", &msg, err)
	}

	j = append(j, byte('\n'))
	ev.json = j

	return j
}

// Constructs the message for a PartialObjectMetadata response
func (ev *WatchEvent) PartialObjectMetadataJSON() []byte {
	ev.mutex.Lock()
	defer ev.mutex.Unlock()

	if ev.partialObjectMetadataJSON != nil {
		return ev.partialObjectMetadataJSON
	}
	u := ev.internalObject

	partialObjectMetadata := &metav1.PartialObjectMetadata{}
	partialObjectMetadata.APIVersion = u.GetAPIVersion()
	partialObjectMetadata.Kind = u.GetKind()

	partialObjectMetadata.APIVersion = "meta.k8s.io/v1beta1"
	partialObjectMetadata.Kind = "PartialObjectMetadata"
	// {"kind":"PartialObjectMetadata","apiVersion":"meta.k8s.io/v1beta1","metadata"":

	partialObjectMetadata.Annotations = u.GetAnnotations()
	partialObjectMetadata.Labels = u.GetLabels()
	partialObjectMetadata.Name = u.GetName()
	partialObjectMetadata.Namespace = u.GetNamespace()
	partialObjectMetadata.ResourceVersion = u.GetResourceVersion()
	partialObjectMetadata.Generation = u.GetGeneration()
	partialObjectMetadata.CreationTimestamp = u.GetCreationTimestamp()
	partialObjectMetadata.DeletionTimestamp = u.GetDeletionTimestamp()
	partialObjectMetadata.DeletionGracePeriodSeconds = u.GetDeletionGracePeriodSeconds()
	partialObjectMetadata.GenerateName = u.GetGenerateName()

	msg := messageV1{
		Type:   ev.eventType,
		Object: partialObjectMetadata,
	}

	j, err := json.Marshal(&msg)
	if err != nil {
		klog.Fatalf("error from json.Marshal(%T): %v", &msg, err)
	}

	j = append(j, byte('\n'))
	ev.partialObjectMetadataJSON = j
	return j
}
