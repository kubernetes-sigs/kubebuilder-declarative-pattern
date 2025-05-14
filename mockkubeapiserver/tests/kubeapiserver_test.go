package applier

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kubebuilder-declarative-pattern/ktest/httprecorder"
	"sigs.k8s.io/kubebuilder-declarative-pattern/ktest/testharness"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver"
)

func TestGoldenTests(t *testing.T) {
	testharness.RunGoldenTests(t, "testdata", func(h *testharness.Harness, testdir string) {
		ctx := context.Background()

		k8s, err := mockkubeapiserver.NewMockKubeAPIServer(":0")
		if err != nil {
			t.Fatalf("error building mock kube-apiserver: %v", err)
		}
		defer func() {
			if err := k8s.Stop(); err != nil {
				t.Fatalf("error closing mock kube-apiserver: %v", err)
			}
		}()

		addr, err := k8s.StartServing()
		if err != nil {
			t.Fatalf("error starting mock kube-apiserver: %v", err)
		}

		klog.Infof("mock kubeapiserver will listen on %v", addr)

		var requestLog httprecorder.RequestLog
		wrapTransport := func(rt http.RoundTripper) http.RoundTripper {
			return httprecorder.NewRecorder(rt, &requestLog)
		}
		restConfig := &rest.Config{
			Host:          addr.String(),
			WrapTransport: wrapTransport,
		}

		httpClient, err := rest.HTTPClientFor(restConfig)
		if err != nil {
			t.Fatalf("error from rest.HTTPClientFor: %v", err)
		}

		p := filepath.Join(testdir, "manifest.yaml")
		manifestYAML := string(h.MustReadFile(p))
		objects, err := testharness.ParseObjects(ctx, manifestYAML)
		if err != nil {
			t.Errorf("error parsing manifest %q: %v", p, err)
		}

		restMapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
		if err != nil {
			t.Fatalf("error from apiutil.NewDynamicRESTMapper: %v", err)
		}

		dynamicClient, err := dynamic.NewForConfigAndClient(restConfig, httpClient)
		if err != nil {
			t.Fatalf("building dynamic client: %v", err)
		}
		for _, obj := range objects {
			gvk := obj.GroupVersionKind()
			restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				t.Errorf("error getting restmapping for %v: %v", gvk, err)
			}

			var applyOptions metav1.ApplyOptions
			applyOptions.FieldManager = "test"
			resource := dynamicClient.Resource(restMapping.Resource).Namespace(obj.GetNamespace())

			if _, err := resource.Apply(ctx, obj.GetName(), obj, applyOptions); err != nil {
				t.Fatalf("error applying resource %v: %v", gvk, err)
			}
		}

		t.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
		requestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
		requestLog.RemoveUserAgent()
		requestLog.ReplaceHeader("Date", "(removed)")
		requestLog.SortGETs()

		requests := requestLog.FormatHTTP(true)
		h.CompareGoldenFile(filepath.Join(testdir, "expected.yaml"), requests)
	})
}
