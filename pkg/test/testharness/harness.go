package testharness

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/restmapper"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/httprecorder"
)

type Harness struct {
	*testing.T
}

func New(t *testing.T) *Harness {
	h := &Harness{T: t}
	t.Cleanup(h.Cleanup)
	return h
}

func (h *Harness) Cleanup() {

}

func (h *Harness) TempDir() string {
	tmpdir, err := os.MkdirTemp("", "test")
	if err != nil {
		h.Fatalf("failed to make temp directory: %v", err)
	}
	h.T.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			h.Errorf("error cleaning up temp directory %q: %v", tmpdir, err)
		}
	})
	return tmpdir
}

func (h *Harness) MustReadFile(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		h.Fatalf("error from ReadFile(%q): %v", p, err)
	}
	return b
}

func (h *Harness) FileExists(p string) bool {
	_, err := os.Stat(p)
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		h.Fatalf("error from os.Stat(%q): %v", p, err)
	}
	return false
}

func (h *Harness) StartKube() *mockkubeapiserver.MockKubeAPIServer {
	k8s, err := mockkubeapiserver.NewMockKubeAPIServer(":0")
	if err != nil {
		h.Fatalf("error building mock kube-apiserver: %v", err)
	}

	h.T.Cleanup(func() {
		if err := k8s.Stop(); err != nil {
			h.Fatalf("error closing mock kube-apiserver: %v", err)
		}
	})

	addr, err := k8s.StartServing()
	if err != nil {
		h.Errorf("error starting mock kube-apiserver: %v", err)
	}
	h.Logf("started mock kube-apiserver on %v", addr)
	return k8s
}

func (h *Harness) NewControllerManager(restConfig *rest.Config, addToSchemeFunctions ...func(*runtime.Scheme) error) ctrl.Manager {
	scheme := runtime.NewScheme()
	for _, addToSchemeFunction := range addToSchemeFunctions {
		if err := addToSchemeFunction(scheme); err != nil {
			h.Fatalf("error from AddToScheme: %v", err)
		}
	}

	logger := klogr.New()

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "",
		Port:               0,
		LeaderElection:     false,

		// MapperProvider provides the rest mapper used to map go types to Kubernetes APIs
		MapperProvider: restmapper.NewControllerRESTMapper,

		Logger: logger,
	})
	if err != nil {
		h.Fatalf("error starting manager: %v", err)
	}

	return mgr
}

func (h *Harness) CompareHTTPLog(expectedPath string, requestLog httprecorder.RequestLog, restConfig *rest.Config) {
	h.Logf("replacing old url prefix %q", "http://"+restConfig.Host)
	requestLog.ReplaceURLPrefix("http://"+restConfig.Host, "http://kube-apiserver")
	requestLog.RemoveUserAgent()
	// Workaround for non-determinism in https://github.com/kubernetes/kubernetes/blob/79a62d62350fb600f97d1f6309c3274515b3587a/staging/src/k8s.io/client-go/tools/cache/reflector.go#L301
	requestLog.RegexReplaceURL("&timeoutSeconds=.*&", "&timeoutSeconds=<replaced>&")

	requests := requestLog.FormatHTTP()
	h.CompareGoldenFile(expectedPath, requests)
}
