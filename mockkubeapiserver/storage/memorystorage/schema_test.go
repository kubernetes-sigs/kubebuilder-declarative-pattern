package memorystorage

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
	"sigs.k8s.io/yaml"
)

func TestApply(t *testing.T) {
	ctx := context.TODO()

	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	memoryStorage, err := NewMemoryStorage(storage.NewTestClock(), storage.NewTestUIDGenerator())
	if err != nil {
		t.Fatalf("NewMemoryStorage failed: %v", err)
	}

	resource := memoryStorage.findResourceByGVK(gvk)
	if resource == nil {
		t.Fatalf("findResourceByGVK(%v) unexpectedly returned nil", gvk)
	}

	liveYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  foo1: bar1
`

	liveObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(liveYAML), liveObj); err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}

	patchYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  foo2: bar2
`

	applyOptions := metav1.PatchOptions{
		FieldManager: "foo",
	}

	mergedObject, changed, err := storage.DoServerSideApply(ctx, resource, liveObj, []byte(patchYAML), applyOptions)
	if err != nil {
		t.Fatalf("DoServerSideApply gave error: %v", err)
	}
	if !changed {
		t.Fatalf("DoServerSideApply indicated object was not changed; expected change")
	}

	gotYAML, err := yaml.Marshal(mergedObject)
	if err != nil {
		t.Fatalf("error marshaling yaml: %v", err)
	}

	want := `
apiVersion: v1
data:
  foo1: bar1
  foo2: bar2
kind: ConfigMap
metadata:
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:apiVersion: {}
      f:data:
        f:foo2: {}
      f:kind: {}
      f:metadata:
        f:name: {}
        f:namespace: {}
    manager: foo
    operation: Apply
  name: cm1
  namespace: default
`

	got := strings.TrimSpace(string(gotYAML))
	want = strings.TrimSpace(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected diff in result: %v", diff)
	}
}
