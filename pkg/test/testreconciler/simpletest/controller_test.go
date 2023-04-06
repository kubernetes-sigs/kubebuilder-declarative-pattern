package simpletest

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/loaders"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/applier"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/httprecorder"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/testharness"

	api "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/testreconciler/simpletest/v1alpha1"
)

func TestSimpleReconciler(t *testing.T) {
	appliers := []struct {
		Key     string
		Applier applier.Applier
		Status  declarative.Status
	}{
		{
			Key:     "direct",
			Applier: applier.NewDirectApplier(),
			Status:  status.NewBasic(nil),
		},
		{
			Key:     "ssa",
			Applier: applier.NewApplySetApplier(metav1.PatchOptions{FieldManager: "kdp-test"}),
			Status:  status.NewKstatusCheck(nil, nil),
		},
	}
	for _, applier := range appliers {
		t.Run(applier.Key, func(t *testing.T) {
			testharness.RunGoldenTests(t, "testdata/reconcile/"+applier.Key+"/", func(h *testharness.Harness, testdir string) {
				testSimpleReconciler(h, testdir, applier.Applier, applier.Status)
			})
		})
	}
}

func testSimpleReconciler(h *testharness.Harness, testdir string, applier applier.Applier, status declarative.Status) {
	ctx := context.Background()

	k8s := h.StartKube()

	k8s.RegisterType(schema.GroupVersionKind{Group: "addons.example.org", Version: "v1alpha1", Kind: "SimpleTest"}, "simpletests", meta.RESTScopeNamespace)

	defer func() {
		if err := k8s.Stop(); err != nil {
			h.Fatalf("error closing mock kube-apiserver: %v", err)
		}
	}()

	var requestLog httprecorder.RequestLog
	wrapTransport := func(rt http.RoundTripper) http.RoundTripper {
		return httprecorder.NewRecorder(rt, &requestLog)
	}
	restConfig := &rest.Config{
		Host:          k8s.ListenerAddress().String(),
		WrapTransport: wrapTransport,
	}

	mgr := h.NewControllerManager(restConfig, api.AddToScheme)

	reconciler := &SimpleTestReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		applier: applier,
		status:  status,
	}

	mc, err := loaders.NewManifestLoader("testdata/channels")
	if err != nil {
		h.Fatalf("error from loaders.NewManifestLoader: %v", err)
	}
	reconciler.manifestController = mc

	if err = reconciler.SetupWithManager(mgr); err != nil {
		h.Fatalf("error creating reconciler: %v", err)
	}

	if h.FileExists(filepath.Join(testdir, "before.yaml")) {
		before := string(h.MustReadFile(filepath.Join(testdir, "before.yaml")))
		if err := k8s.AddObjectsFromManifest(before); err != nil {
			h.Fatalf("error precreating objects: %v", err)
		}
	}

	mgrContext, mgrStop := context.WithCancel(ctx)
	go func() {
		time.Sleep(1 * time.Second)
		// time.Sleep(200 * time.Second)
		mgrStop()
	}()
	if err := mgr.Start(mgrContext); err != nil {
		h.Fatalf("error starting manager: %v", err)
	}

	h.CompareHTTPLog(filepath.Join(testdir, "expected-http.yaml"), &requestLog, restConfig)
}
