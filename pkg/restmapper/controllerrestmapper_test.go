package restmapper

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func TestRESTMapping(t *testing.T) {
	// TODO: Add mock
	home := homedir.HomeDir()
	kubeconfigPath := filepath.Join(home, ".kube", "config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(err)
	}

	restMapper, err := NewControllerRESTMapper(restConfig)
	if err != nil {
		t.Fatalf("error from NewControllerRESTMapper: %v", err)
	}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		t.Fatalf("error from RESTMapping(%v): %v", gvk, err)
	}

	got := fmt.Sprintf("resource:%v\ngvk:%v\nscope:%v", restMapping.Resource, restMapping.GroupVersionKind, restMapping.Scope.Name())
	want := `
resource:/v1, Resource=namespaces
gvk:/v1, Kind=Namespace
scope:root
`
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RESTMapping(%v) diff (-want +got):\n%s", gvk, diff)
	}
}
