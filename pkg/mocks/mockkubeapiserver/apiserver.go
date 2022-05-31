package mockkubeapiserver

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

func NewMockKubeAPIServer(addr string) (*MockKubeAPIServer, error) {
	s := &MockKubeAPIServer{}
	if addr == "" {
		addr = ":http"
	}

	s.httpServer = &http.Server{Addr: addr, Handler: s}

	return s, nil
}

type MockKubeAPIServer struct {
	httpServer *http.Server
	listener   net.Listener

	schema mockSchema
}

type mockSchema struct {
	resources []mockSchemaResource
}

type mockSchemaResource struct {
	metav1.APIResource
}

func (s *MockKubeAPIServer) StartServing() (net.Addr, error) {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return nil, err
	}
	s.listener = listener
	addr := listener.Addr()
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil {
			if err != http.ErrServerClosed {
				klog.Errorf("error serving: %v", err)
			}
		}
	}()
	return addr, nil
}

func (s *MockKubeAPIServer) Stop() error {
	return s.httpServer.Close()
}

func (s *MockKubeAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	tokens := strings.Split(strings.Trim(path, "/"), "/")
	if len(tokens) == 2 {
		if tokens[0] == "api" && tokens[1] == "v1" {
			switch r.Method {
			case http.MethodGet:
				req := &apiResourcesRequest{}
				req.Init(w, r)

				err := req.Run(s)
				if err != nil {
					klog.Warningf("internal error for %s %s: %v", r.Method, r.URL, err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}

				return
			default:
				klog.Warningf("method not allowed for %s %s", r.Method, r.URL)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			}
		}
	}
	klog.Warningf("404 for %s %s tokens=%#v", r.Method, r.URL, tokens)
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

// baseRequest is the base for our higher-level http requests
type baseRequest struct {
	w http.ResponseWriter
	r *http.Request
}

func (b *baseRequest) Init(w http.ResponseWriter, r *http.Request) {
	b.w = w
	b.r = r
}

func (r *baseRequest) writeResponse(obj interface{}) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error from json.Marshal on %T: %w", obj, err)
	}
	if _, err := r.w.Write(b); err != nil {
		// Too late to send error response
		klog.Warningf("error writing http response: %w", err)
		return nil
	}
	return nil
}

// apiResourcesRequest is a wrapper around a request to list APIResources, such as /api/v1
type apiResourcesRequest struct {
	baseRequest
}

// doGetAPIV1 serves the GET /api/v1 endpoint
func (r *apiResourcesRequest) Run(s *MockKubeAPIServer) error {
	resourceList := &metav1.APIResourceList{}

	for _, resource := range s.schema.resources {
		apiResource := resource.APIResource
		resourceList.APIResources = append(resourceList.APIResources, apiResource)
	}

	return r.writeResponse(resourceList)
}

// Add registers a type with the schema for the mock kubeapiserver
func (s *MockKubeAPIServer) Add(gvk schema.GroupVersionKind, resource string, scope meta.RESTScope) {
	r := mockSchemaResource{
		APIResource: metav1.APIResource{
			Name:    resource,
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
	}
	if scope.Name() == meta.RESTScopeNameNamespace {
		r.Namespaced = true
	}

	s.schema.resources = append(s.schema.resources, r)
}
