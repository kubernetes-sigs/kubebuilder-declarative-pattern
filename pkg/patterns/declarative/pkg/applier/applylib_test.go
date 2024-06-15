package applier

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/kubebuilder-declarative-pattern/applylib/applyset"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/restmapper"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/httprecorder"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/testharness"
)

func fakeParent() runtime.Object {
	parent := &unstructured.Unstructured{}
	parent.SetKind("ConfigMap")
	parent.SetAPIVersion("v1")
	parent.SetName("test")
	parent.SetNamespace("default")
	return parent
}

func TestApplySetApplier(t *testing.T) {
	patchOptions := metav1.PatchOptions{FieldManager: "kdp-test"}
	applier := NewApplySetApplier(patchOptions, metav1.DeleteOptions{}, ApplysetOptions{})
	runApplierGoldenTests(t, "testdata/applylib", false, applier)
}

func runApplierGoldenTests(t *testing.T, testDir string, interceptHTTPServer bool, applier Applier) {
	testharness.RunGoldenTests(t, testDir, func(h *testharness.Harness, testdir string) {
		ctx := context.Background()

		k8s, err := mockkubeapiserver.NewMockKubeAPIServer(":0")
		if err != nil {
			t.Fatalf("error building mock kube-apiserver: %v", err)
		}

		k8s.RegisterType(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, "namespaces", meta.RESTScopeRoot)

		defer func() {
			if err := k8s.Stop(); err != nil {
				t.Fatalf("error closing mock kube-apiserver: %v", err)
			}
		}()

		addr, err := k8s.StartServing()
		if err != nil {
			t.Errorf("error starting mock kube-apiserver: %v", err)
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

		var apiserverRequestLog httprecorder.RequestLog
		if interceptHTTPServer {
			k8s.AddHook(&logKubeRequestsHook{log: &apiserverRequestLog})
		}

		if h.FileExists(filepath.Join(testdir, "before.yaml")) {
			before := string(h.MustReadFile(filepath.Join(testdir, "before.yaml")))
			if err := k8s.AddObjectsFromManifest(before); err != nil {
				t.Fatalf("error precreating objects: %v", err)
			}
		}
		p := filepath.Join(testdir, "manifest.yaml")
		manifestYAML := string(h.MustReadFile(p))
		objects, err := manifest.ParseObjects(ctx, manifestYAML)
		if err != nil {
			t.Errorf("error parsing manifest %q: %v", p, err)
		}

		restMapper, err := restmapper.NewForTest(restConfig)
		if err != nil {
			t.Fatalf("error from controllerrestmapper.NewForTest: %v", err)
		}

		parent := fakeParent()
		gvk := parent.GetObjectKind().GroupVersionKind()
		restmapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			t.Errorf("error getting restmapping for parent %v", err)
		}

		client := fake.NewClientBuilder().WithRuntimeObjects(parent).Build()
		options := ApplierOptions{
			Objects:    objects.GetItems(),
			RESTConfig: restConfig,
			RESTMapper: restMapper,
			ParentRef:  applyset.NewParentRef(parent, "test", "default", restmapping),
			Client:     client,
		}
		if err := applier.Apply(ctx, options); err != nil {
			t.Fatalf("error from applier.Apply: %v", err)
		}

		t.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
		requestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
		requestLog.RemoveUserAgent()
		requestLog.SortGETs()

		requests := requestLog.FormatHTTP(false)
		h.CompareGoldenFile(filepath.Join(testdir, "expected.yaml"), requests)

		if interceptHTTPServer {
			t.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
			apiserverRequestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
			apiserverRequestLog.RemoveUserAgent()
			apiserverRequestLog.SortGETs()
			apiserverRequestLog.RemoveHeader("Kubectl-Session")
			apiserverRequests := apiserverRequestLog.FormatHTTP(false)
			h.CompareGoldenFile(filepath.Join(testdir, "expected-apiserver.yaml"), apiserverRequests)
		}
	})
}

// logKubeRequestsHook is a hook to record mock-kubeapiserver requests to a RequestLog
type logKubeRequestsHook struct {
	log *httprecorder.RequestLog
}

var _ mockkubeapiserver.BeforeHTTPOperation = &logKubeRequestsHook{}

func (h *logKubeRequestsHook) BeforeHTTPOperation(op *mockkubeapiserver.HTTPOperation) {
	req := op.Request
	entry := &httprecorder.LogEntry{}
	entry.Request = httprecorder.Request{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header,
	}

	if req.Body != nil {
		requestBody, err := io.ReadAll(req.Body)
		if err != nil {
			panic("failed to read request body")
		}
		entry.Request.Body = string(requestBody)
		req.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
	h.log.AddEntry(entry)
}
