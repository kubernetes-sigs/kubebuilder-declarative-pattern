package simpletest

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/loaders"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/applier"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/restmapper"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target/remotetargethook"
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
		time.Sleep(2 * time.Second)
		// time.Sleep(200 * time.Second)
		mgrStop()
	}()
	if err := mgr.Start(mgrContext); err != nil {
		h.Fatalf("error starting manager: %v", err)
	}

	h.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
	requestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
	requestLog.RemoveUserAgent()
	// Workaround for non-determinism in https://github.com/kubernetes/kubernetes/blob/79a62d62350fb600f97d1f6309c3274515b3587a/staging/src/k8s.io/client-go/tools/cache/reflector.go#L301
	requestLog.RegexReplaceURL("&timeoutSeconds=.*&", "&timeoutSeconds=<replaced>&")

	requests := requestLog.FormatHTTP()

	h.CompareGoldenFile(filepath.Join(testdir, "expected-http.yaml"), requests)
}

func TestOverrideTarget(t *testing.T) {
	appliers := []struct {
		Key     string
		Applier applier.Applier
	}{
		{
			Key:     "direct",
			Applier: applier.NewDirectApplier(),
		},
		{
			Key:     "ssa",
			Applier: applier.NewApplySetApplier(metav1.PatchOptions{FieldManager: "kdp-test"}),
		},
	}
	for _, applier := range appliers {
		t.Run(applier.Key, func(t *testing.T) {
			testharness.RunGoldenTests(t, "testdata/overridetarget/"+applier.Key+"/", func(h *testharness.Harness, testdir string) {
				testOverrideTarget(h, testdir, applier.Applier)
			})
		})
	}
}

func testOverrideTarget(h *testharness.Harness, testdir string, applier applier.Applier) {
	ctx := context.Background()

	primary := h.StartKube()
	primary.RegisterType(schema.GroupVersionKind{Group: "addons.example.org", Version: "v1alpha1", Kind: "SimpleTest"}, "simpletests", meta.RESTScopeNamespace)

	targetK8s := h.StartKube()

	var primaryRequestLog httprecorder.RequestLog
	primaryWrapTransport := func(rt http.RoundTripper) http.RoundTripper {
		return httprecorder.NewRecorder(rt, &primaryRequestLog)
	}
	primaryRestConfig := &rest.Config{
		Host:          primary.ListenerAddress().String(),
		WrapTransport: primaryWrapTransport,
	}

	var targetRequestLog httprecorder.RequestLog
	targetWrapTransport := func(rt http.RoundTripper) http.RoundTripper {
		return httprecorder.NewRecorder(rt, &targetRequestLog)
	}
	targetRestConfig := &rest.Config{
		Host:          targetK8s.ListenerAddress().String(),
		WrapTransport: targetWrapTransport,
	}

	mgr := h.NewControllerManager(primaryRestConfig, api.AddToScheme)

	reconciler := &SimpleTestReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		applier: applier,
	}

	mc, err := loaders.NewManifestLoader("testdata/channels")
	if err != nil {
		h.Fatalf("error from loaders.NewManifestLoader: %v", err)
	}
	reconciler.manifestController = mc

	targetCache := target.NewCache()

	targetRestMapper, err := restmapper.NewControllerRESTMapper(targetRestConfig)
	if err != nil {
		h.Fatalf("error from restmapper.NewControllerRESTMapper: %v", err)
	}
	targetResolver := &testTargetResolver{
		key: "remote1",
		restInfo: target.RESTInfo{
			RESTConfig: targetRestConfig,
			RESTMapper: targetRestMapper,
		},
	}
	remoteTargetHook := remotetargethook.NewRemoteTargetHook(targetResolver, targetCache)

	if err = reconciler.SetupWithManager(mgr, declarative.WithHook(remoteTargetHook)); err != nil {
		h.Fatalf("error from SetupWithManager: %v", err)
	}

	if h.FileExists(filepath.Join(testdir, "before.yaml")) {
		before := string(h.MustReadFile(filepath.Join(testdir, "before.yaml")))
		if err := primary.AddObjectsFromManifest(before); err != nil {
			h.Fatalf("error precreating objects: %v", err)
		}
	}

	mgrContext, mgrStop := context.WithCancel(ctx)
	go func() {
		time.Sleep(2 * time.Second)
		// time.Sleep(200 * time.Second)
		mgrStop()
	}()
	if err := mgr.Start(mgrContext); err != nil {
		h.Fatalf("error starting manager: %v", err)
	}

	h.CompareHTTPLog(filepath.Join(testdir, "expected-primary-http.yaml"), primaryRequestLog, primaryRestConfig)
	h.CompareHTTPLog(filepath.Join(testdir, "expected-target-http.yaml"), targetRequestLog, targetRestConfig)
}

type testTargetResolver struct {
	key      string
	restInfo target.RESTInfo
}

func (r *testTargetResolver) ResolveKey(ctx context.Context, subject client.Object) (string, bool, error) {
	return r.key, true, nil
}
func (r *testTargetResolver) Resolve(ctx context.Context, subject client.Object, key string) (*target.RESTInfo, error) {
	if key != r.key {
		return nil, fmt.Errorf("key %q not known", key)
	}
	return &r.restInfo, nil
}

var _ remotetargethook.RemoteTargetResolver = &testTargetResolver{}
