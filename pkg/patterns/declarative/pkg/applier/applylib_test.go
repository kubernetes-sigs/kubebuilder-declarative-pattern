package applier

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver"
	controllerrestmapper "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/restmapper"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/httprecorder"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/testharness"
)

func TestApplySetApplier(t *testing.T) {
	patchOptions := metav1.PatchOptions{FieldManager: "kdp-test"}
	applier := NewApplySetApplier(patchOptions)
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
		manifest := string(h.MustReadFile(filepath.Join(testdir, "manifest.yaml")))

		restMapper, err := controllerrestmapper.NewControllerRESTMapper(restConfig)
		if err != nil {
			t.Fatalf("error from controllerrestmapper.NewControllerRESTMapper: %v", err)
		}

		options := ApplierOptions{
			Manifest:   manifest,
			RESTConfig: restConfig,
			RESTMapper: restMapper,
		}
		if err := applier.Apply(ctx, options); err != nil {
			t.Fatalf("error from applier.Apply: %v", err)
		}

		t.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
		requestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
		requestLog.RemoveUserAgent()
		requests := requestLog.FormatHTTP()
		h.CompareGoldenFile(filepath.Join(testdir, "expected.yaml"), requests)

		if interceptHTTPServer {
			t.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
			apiserverRequestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
			apiserverRequestLog.RemoveUserAgent()
			apiserverRequestLog.RemoveHeader("Kubectl-Session")
			apiserverRequests := apiserverRequestLog.FormatHTTP()
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
	entry := httprecorder.LogEntry{}
	entry.Request = httprecorder.Request{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header,
	}

	if req.Body != nil {
		requestBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic("failed to read request body")
		}
		entry.Request.Body = string(requestBody)
		req.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
	}
	h.log.Entries = append(h.log.Entries, entry)
}
