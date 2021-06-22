/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package applier

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	//"time"

	//appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	//diskcached "k8s.io/client-go/discovery/cached/disk"
	memorycached "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestDirectApply(t *testing.T) {
	tests := []struct {
		name              string
		namespace         string
		expectedNamespace string
		manifest          string
		validate          bool
		isErr             bool
		expectedErrStr    string
		args              []string
	}{
		{
			//TODO: IMHO, it is needed to pass this test w/ all applier. We should update direct.go
			name:              "namespace defaulting",
			namespace:         "",
			expectedNamespace: apiv1.NamespaceDefault,
			manifest: `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  labels:
    app: test-app
spec:
  selector:
    matchLabels:
      app: guestbook
      tier: frontend
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4`,
		},
		{
			name:              "normal manifest",
			namespace:         "",
			expectedNamespace: "normal-manifest-ns",
			manifest: `---
apiVersion: v1
kind: Namespace
metadata:
  name: normal-manifest-ns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: normal-manifest-ns
  labels:
    app: test-app
spec:
  selector:
    matchLabels:
      app: guestbook
      tier: frontend
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4`,
		},
		{
			name:              "validation error",
			namespace:         "",
			expectedNamespace: "validation-error-ns",
			validate:          true,
			isErr:             true,
			expectedErrStr:    "error validating data: ValidationError(Deployment.metadata): unknown field \"invalid-field\"",
			manifest: `---
apiVersion: v1
kind: Namespace
metadata:
  name: validation-error-ns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: validation-error-ns
  invalid-field: invalid-value
  labels:
    app: test-app
spec:
  selector:
    matchLabels:
      app: guestbook
      tier: frontend
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4`,
		},
		{
			//TODO: IMHO, it is needed to pass this test w/ all applier. We should update direct.go
			name:              "preserve namespace apply",
			namespace:         "preserve-namespace-apply-ns", // case of using PreserveNamespace()
			expectedNamespace: "preserve-namespace-apply-ns",
			validate:          false,
			isErr:             false,
			manifest: `---
apiVersion: v1
kind: Namespace
metadata:
  name: preserve-namespace-apply-ns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
# namespace will be set to "preserve-namespace-apply-ns" by Apply
  labels:
    app: test-app
spec:
  selector:
    matchLabels:
      app: guestbook
      tier: frontend
  replicas: 3
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: gcr.io/google-samples/gb-frontend:v4`,
		},
	}
	testenv := &envtest.Environment{}
	cfg, err := testenv.Start()
	defer testenv.Stop()
	if err != nil {
		t.Errorf("fail to start envtest k8s cluster: %v", err)
	}
	direct := NewDirectApplier()
	direct.ConfigFlags = &DummyRESTClientGetter{config: cfg}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := direct.Apply(context.Background(), test.namespace, test.manifest, test.validate, test.args...)
			if test.isErr == false {
				if err != nil {
					t.Errorf("fail to apply: %v", err)
				}
				clientset, err := kubernetes.NewForConfig(cfg)
				if err != nil {
					t.Errorf("fail to create clientset: %v", err)
				}
				result, err := clientset.AppsV1().Deployments(test.expectedNamespace).Get(context.Background(), "frontend", metav1.GetOptions{})
				//result, err := clientset.AppsV1().Deployments(apiv1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
				//apiv1.NamespaceDefault
				if err != nil {
					t.Errorf("fail to get resource: %v", err)
				}

				if result.ObjectMeta.Namespace != test.expectedNamespace {
					t.Errorf("namespace mismatch. actual: %v, expected: %v", result.ObjectMeta.Namespace, test.expectedNamespace)
				}
			} else {
				// failure something like a validation error.
				errStr := fmt.Sprintf("%v", err)
				if strings.Contains(errStr, test.expectedErrStr) == false {
					t.Errorf("occured error is not expected error: %v", err)
				}
			}
		})
	}
}

// DummyRESTClientGetter implements RESTClientGetter interface.
// Almost part uses codes on k8s.io/cli-runtime/pkg/genericclioptions/config_flags.go
type DummyRESTClientGetter struct {
	config *rest.Config
}

// ToRESTConfig implements RESTClientGetter.
func (d *DummyRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return d.config, nil
}

// ToRawKubeConfigLoader binds config flag values to config overrides
func (d *DummyRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return d.toRawKubeConfigLoader()
}

func (d *DummyRESTClientGetter) toRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	// bind auth info flag values to overrides
	if d.config.CertFile != "" {
		overrides.AuthInfo.ClientCertificate = d.config.CertFile
	}
	if d.config.KeyFile != "" {
		overrides.AuthInfo.ClientKey = d.config.KeyFile
	}
	if d.config.BearerToken != "" {
		overrides.AuthInfo.Token = d.config.BearerToken
	}
	if d.config.Impersonate.UserName != "" {
		overrides.AuthInfo.Impersonate = d.config.Impersonate.UserName
	}
	if d.config.Impersonate.Groups != nil {
		overrides.AuthInfo.ImpersonateGroups = d.config.Impersonate.Groups
	}
	if d.config.Username != "" {
		overrides.AuthInfo.Username = d.config.Username
	}
	if d.config.Password != "" {
		overrides.AuthInfo.Password = d.config.Password
	}

	// bind cluster flags
	if d.config.Host != "" {
		overrides.ClusterInfo.Server = d.config.Host
	}
	if d.config.TLSClientConfig.ServerName != "" {
		overrides.ClusterInfo.TLSServerName = d.config.TLSClientConfig.ServerName
	}
	if d.config.CAFile != "" {
		overrides.ClusterInfo.CertificateAuthority = d.config.CAFile
	}
	overrides.ClusterInfo.InsecureSkipTLSVerify = d.config.Insecure
	//if d.config.Insecure != nil {
	//		overrides.ClusterInfo.InsecureSkipTLSVerify = d.config.Insecure
	//}

	// bind context flags
	//if d.Context != nil {
	//	overrides.CurrentContext = *d.Context
	//}
	//if d.ClusterName != nil {
	//	overrides.Context.Cluster = *d.ClusterName
	//}
	//if d.AuthInfoName != nil {
	//	overrides.Context.AuthInfo = *d.AuthInfoName
	//}
	//if d.Namespace != nil {
	//	overrides.Context.Namespace = *d.Namespace
	//}
	//overrides.Context.Namespace = "hogehoge"

	if d.config.Timeout.String() != "" {
		overrides.Timeout = d.config.Timeout.String()
	}

	// we only have an interactive prompt when a password is allowed
	if d.config.Password == "" {
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	}
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin)
}

// ToDiscoveryClient implements RESTClientGetter.
func (d *DummyRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := d.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = 100

	//cacheDir := defaultCacheDir

	//// retrieve a user-provided value for the "cache-dir"
	//// override httpCacheDir and discoveryCacheDir if user-value is given.
	//if d.CacheDir != nil {
	//	cacheDir = *d.CacheDir
	//}
	//httpCacheDir := filepath.Join(cacheDir, "http")
	//discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	//return diskcached.NewCachedDiscoveryClientForConfig(config, "", "", time.Duration(10*time.Minute))
	return memorycached.NewMemCacheClient(discovery.NewDiscoveryClientForConfigOrDie(config)), nil
	//return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))
}

// ToRESTMapper returns a mapper.
func (d *DummyRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := d.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}
