package applyset

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	kubectlapply "sigs.k8s.io/kubebuilder-declarative-pattern/applylib/forked/github.com/kubernetes/kubectl/pkg/cmd/apply"
)

type BeforePruneHook func(ctx context.Context, op BeforePrune) error

type Object interface {
	GroupVersionKind() schema.GroupVersionKind
	GetNamespace() string
	GetName() string

	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

type BeforePrune struct {
	pruneObjects *ObjectSet
}

func (op *BeforePrune) PruneObjects() *ObjectSet {
	return op.pruneObjects
}

type ObjectSet struct {
	objects []*hookObject
}

func newObjectSet(objects ...kubectlapply.PruneObject) *ObjectSet {
	s := &ObjectSet{}
	for _, obj := range objects {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			klog.Fatalf("failed to get access for object %T: %v", obj, err)
		}

		s.objects = append(s.objects, &hookObject{accessor: accessor, obj: obj})
	}
	return s
}

func (s *ObjectSet) VisitObjects(fn func(obj Object)) {
	for _, obj := range s.objects {
		fn(obj)
	}
}

type hookObject struct {
	obj      kubectlapply.PruneObject
	accessor metav1.Object
}

func (o *hookObject) GetName() string {
	return o.accessor.GetName()
}

func (o *hookObject) GetNamespace() string {
	return o.accessor.GetNamespace()
}

func (o *hookObject) GroupVersionKind() schema.GroupVersionKind {
	return o.obj.Mapping.GroupVersionKind
}

func (o *hookObject) GetAnnotations() map[string]string {
	return o.accessor.GetAnnotations()
}

func (o *hookObject) GetLabels() map[string]string {
	return o.accessor.GetLabels()
}
