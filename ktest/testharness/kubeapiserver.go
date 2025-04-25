package testharness

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kubebuilder-declarative-pattern/ktest/httprecorder"
	"sigs.k8s.io/yaml"
)

type TestKubeAPIServer struct {
	t          *testing.T
	ctx        context.Context
	restConfig *rest.Config
	client     client.Client
}

func NewTestKubeAPIServer(t *testing.T, ctx context.Context, env *envtest.Environment) *TestKubeAPIServer {
	s := &TestKubeAPIServer{t: t, ctx: ctx}

	if env != nil {
		restConfig, err := env.Start()
		if err != nil {
			t.Fatalf("failed to start envtest kube-apiserver: %v", err)
		}
		s.restConfig = restConfig
		t.Cleanup(func() {
			if err := env.Stop(); err != nil {
				t.Errorf("error stopping envtest: %v", err)
			}
		})
	} else {
		// Removed for now...
		t.Fatalf("mockkubeapiserver is not supported with this test harness")
		// k8s, err := mockkubeapiserver.NewMockKubeAPIServer(":0")
		// if err != nil {
		// 	t.Fatalf("error building mock kube-apiserver: %v", err)
		// }

		// k8s.RegisterType(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, "namespaces", meta.RESTScopeRoot)

		// defer func() {
		// 	if err := k8s.Stop(); err != nil {
		// 		t.Fatalf("error closing mock kube-apiserver: %v", err)
		// 	}
		// }()

		// addr, err := k8s.StartServing()
		// if err != nil {
		// 	t.Errorf("error starting mock kube-apiserver: %v", err)
		// }

		// klog.Infof("mock kubeapiserver will listen on %v", addr)
		// restConfig := &rest.Config{
		// 	Host: addr.String(),
		// }
		// s = &TestKubeAPIServer{t: t, restConfig: restConfig}
	}

	client, err := client.New(s.restConfig, client.Options{})
	if err != nil {
		t.Fatalf("error creating k8s client: %v", err)
	}
	s.client = client

	return s
}

func (s *TestKubeAPIServer) RESTConfig() *rest.Config {
	c := *s.restConfig
	c.TLSClientConfig = *c.TLSClientConfig.DeepCopy()
	return &c
}

func (s *TestKubeAPIServer) Client() client.Client {
	return s.client
}

// AddProxyAndRecordToLog starts a proxy server that records requests and responses to the log.
// It changes the rest.Config to point to the proxy server.
func (s *TestKubeAPIServer) AddProxyAndRecordToLog(log *httprecorder.RequestLog) {
	proxy := &proxy{
		t:        s.t,
		log:      log,
		upstream: s.restConfig,
	}
	s.restConfig = proxy.Start()

	s.t.Cleanup(proxy.Stop)
}

// AddObject pre-creates an object
func (s *TestKubeAPIServer) AddObject(obj *unstructured.Unstructured) error {
	t := s.t
	t.Logf("precreating %s object %s/%s", obj.GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())

	return s.client.Create(s.ctx, obj)
}

// AddObjectsFromManifest pre-creates the objects in the manifest
func (s *TestKubeAPIServer) AddObjectsFromManifest(y string) error {
	for _, obj := range strings.Split(y, "\n---\n") {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(obj), &u.Object); err != nil {
			return fmt.Errorf("failed to unmarshal object %q: %w", obj, err)
		}
		if err := s.AddObject(u); err != nil {
			return err
		}
	}
	return nil
}

// proxy is a simple http server that proxies requests to the upstream kube-apiserver
// and records the request and response into the log.
// It is used by AddProxyAndRecordToLog
type proxy struct {
	t        *testing.T
	log      *httprecorder.RequestLog
	upstream *rest.Config

	listener        net.Listener
	httpServer      *http.Server
	upstreamBaseURL *url.URL

	httpRecorder *httprecorder.HTTPRecorder
}

// Start starts the proxy server
func (p *proxy) Start() *rest.Config {
	t := p.t
	// Run a simple http server that proxies requests to the upstream server
	httpServer := http.Server{
		Handler: p,
	}

	upstreamBaseURL, err := url.Parse(p.upstream.Host)
	if err != nil {
		t.Fatalf("parsing upstream host: %v", err)
	}
	p.upstreamBaseURL = upstreamBaseURL

	upstreamHTTPClient, err := rest.HTTPClientFor(p.upstream)
	if err != nil {
		t.Fatalf("creating upstream HTTP client: %v", err)
	}
	// By reusing httprecorder.NewRecorder, we share the code to record the request and response.
	p.httpRecorder = httprecorder.NewRecorder(upstreamHTTPClient.Transport, p.log)

	p.httpServer = &httpServer

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("starting proxy: %v", err)
	}
	p.listener = listener
	go httpServer.Serve(listener)

	return &rest.Config{
		Host: listener.Addr().String(),
	}
}

// ServeHTTP is the method that implements the proxy server;
// we proxy requests to the upstream kube-apiserver and record the request and response into the log.
func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := log.FromContext(r.Context())

	upstreamURL := p.upstreamBaseURL.JoinPath(r.URL.Path)
	upstreamURL.RawQuery = r.URL.RawQuery

	upstreamReq := &http.Request{
		Method: r.Method,
		URL:    upstreamURL,
		Header: r.Header,
		Body:   r.Body,
	}

	log.Info("proxying kube-apiserver request", "method", upstreamReq.Method, "url", upstreamURL)
	resp, err := p.httpRecorder.RoundTrip(upstreamReq)
	if err != nil {
		p.t.Fatalf("proxying request: %v", err)
	}
	defer resp.Body.Close()

	// Forward the response headers
	for k, vv := range resp.Header {
		w.Header()[k] = vv
	}
	w.WriteHeader(resp.StatusCode)

	if resp.Body != nil {
		if _, err := io.Copy(w, resp.Body); err != nil {
			p.t.Fatalf("error copying response body: %v", err)
		}
	}
}

// Stop stops the proxy server
func (p *proxy) Stop() {
	p.listener.Close()
}
