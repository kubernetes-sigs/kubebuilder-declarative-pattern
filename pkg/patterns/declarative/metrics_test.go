/*
Copyright 2020 The Kubernetes Authors.

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

package declarative

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"github.com/gtracer/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
	"sigs.k8s.io/yaml"
)

// This test checks gvkString function
func TestGVKString(t *testing.T) {
	testCases := []struct {
		subtest string
		gvk     schema.GroupVersionKind
		want    string
	}{
		{
			subtest: "v1/Pod",
			gvk:     core.SchemeGroupVersion.WithKind("Pod"),
			want:    "v1/Pod",
		},
		{
			subtest: "apps/v1/Deployment",
			gvk:     apps.SchemeGroupVersion.WithKind("Deployment"),
			want:    "apps/v1/Deployment",
		},
	}

	for _, st := range testCases {
		t.Run(st.subtest, func(t *testing.T) {
			if got := gvkString(st.gvk); st.want != got {
				t.Errorf("want:\n%v\ngot:\n%v\n", st.want, got)
			}
		})
	}
}

// This test checks reconcileMetricsFor function & reconcieMetrics.reconcileWith method
func TestReconcileWith(t *testing.T) {
	testCases := []struct {
		subtest    string
		gvks       []schema.GroupVersionKind
		namespaces []string
		names      []string
		want       []string
	}{
		{
			subtest:    "core",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Pod")},
			namespaces: []string{"ns1"},
			names:      []string{"n1"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_count How many times reconciliation of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_count counter
			declarative_reconciler_reconcile_count {group_version_kind = "v1/Pod", name = "n1", namespace = "ns1"} 2
			`,
			},
		},
		{
			subtest:    "core&app",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Pod"), apps.SchemeGroupVersion.WithKind("Deployment")},
			namespaces: []string{"ns1", ""},
			names:      []string{"n1", "n2"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_count How many times reconciliation of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_count counter
			declarative_reconciler_reconcile_count {group_version_kind = "v1/Pod", name = "n1", namespace = "ns1"} 2
			`,
				`
			# HELP declarative_reconciler_reconcile_count How many times reconciliation of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_count counter
			declarative_reconciler_reconcile_count {group_version_kind = "apps/v1/Deployment", name = "n2", namespace = ""} 2
			`,
			},
		},
		{
			subtest:    "node - cluster scoped only",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Node")},
			namespaces: []string{""},
			names:      []string{"n1"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_count How many times reconciliation of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_count counter
			declarative_reconciler_reconcile_count {group_version_kind = "v1/Node", name = "n1", namespace = ""} 2
			`,
			},
		},
	}

	for _, st := range testCases {
		t.Run(st.subtest, func(t *testing.T) {
			for i, gvk := range st.gvks {
				rm := reconcileMetricsFor(gvk)

				rm.reconcileWith(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: st.namespaces[i], Name: st.names[i]}})
				rm.reconcileWith(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: st.namespaces[i], Name: st.names[i]}})

				if err := testutil.CollectAndCompare(rm.reconcileCounterVec.WithLabelValues(gvkString(gvk),
					st.namespaces[i], st.names[i]), strings.NewReader(st.want[i])); err != nil {

					t.Error(err)
				}
			}
		})

		reconcileCount.Reset()
	}
}

// This test checks reconcileMetricsFor function & reconcileMetrics.reconcileFailedWith method
func TestReconcileFailedWith(t *testing.T) {
	testCases := []struct {
		subtest    string
		gvks       []schema.GroupVersionKind
		errs       []error
		namespaces []string
		names      []string
		want       []string
	}{
		{
			subtest:    "core",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Pod")},
			errs:       []error{errors.New("test")},
			namespaces: []string{"ns1"},
			names:      []string{"n1"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_failure_count How many times reconciliation failure of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_failure_count counter
			declarative_reconciler_reconcile_failure_count {group_version_kind = "v1/Pod", name = "n1", namespace = "ns1"} 2
			`,
			},
		},
		{
			subtest:    "core&app",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Pod"), apps.SchemeGroupVersion.WithKind("Deployment")},
			errs:       []error{errors.New("test"), errors.New("test")},
			namespaces: []string{"ns1", ""},
			names:      []string{"n1", "n2"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_failure_count How many times reconciliation failure of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_failure_count counter
			declarative_reconciler_reconcile_failure_count {group_version_kind = "v1/Pod", name = "n1", namespace = "ns1"} 2
			`,
				`
			# HELP declarative_reconciler_reconcile_failure_count How many times reconciliation failure of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_failure_count counter
			declarative_reconciler_reconcile_failure_count {group_version_kind = "apps/v1/Deployment", name = "n2", namespace = ""} 2
			`,
			},
		},
		{
			subtest:    "node - cluster scoped only",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Node")},
			errs:       []error{errors.New("test")},
			namespaces: []string{""},
			names:      []string{"n1"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_failure_count How many times reconciliation failure of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_failure_count counter
			declarative_reconciler_reconcile_failure_count {group_version_kind = "v1/Node", name = "n1", namespace = ""} 2
			`,
			},
		},
		{
			subtest:    "no error",
			gvks:       []schema.GroupVersionKind{core.SchemeGroupVersion.WithKind("Node")},
			errs:       []error{nil},
			namespaces: []string{""},
			names:      []string{"n1"},
			want: []string{`
			# HELP declarative_reconciler_reconcile_failure_count How many times reconciliation failure of K8s objects managed by declarative reconciler occurs
			# TYPE declarative_reconciler_reconcile_failure_count counter
			declarative_reconciler_reconcile_failure_count {group_version_kind = "v1/Node", name = "n1", namespace = ""} 0
			`,
			},
		},
	}

	for _, st := range testCases {
		t.Run(st.subtest, func(t *testing.T) {
			for i, gvk := range st.gvks {
				rm := reconcileMetricsFor(gvk)

				rm.reconcileFailedWith(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: st.namespaces[i], Name: st.names[i]}},
					reconcile.Result{}, st.errs[i])
				rm.reconcileFailedWith(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: st.namespaces[i], Name: st.names[i]}},
					reconcile.Result{}, st.errs[i])

				if err := testutil.CollectAndCompare(rm.reconcileFailureCounterVec.WithLabelValues(gvkString(gvk),
					st.namespaces[i], st.names[i]), strings.NewReader(st.want[i])); err != nil {

					t.Error(err)
				}
			}
		})

		reconcileFailure.Reset()
	}
}

// This test checks *ObjectTracker.addIfNotPresent method
//
// envtest package used in this test requires control
// plane binaries (etcd & kube-apiserver & kubectl).
// The default path these binaries reside in is set to
// /usr/local/kubebuilder/bin .
// This path can be set through environment variable
// KUBEBUILDER_ASSETS .
// It is recommended to download kubebuilder release binaries
// and point that path.
func TestAddIfNotPresent(t *testing.T) {
	const defKubectlPath = "/usr/local/kubebuilder/bin"
	var kubectlPath string

	// Run local kube-apiserver & etecd
	testEnv := envtest.Environment{}
	restConf, err := testEnv.Start()
	if err != nil {
		t.Log("Maybe, you have to make sure control plane binaries" + " " +
			"(kube-apiserver, etcd & kubectl) reside in" + " " +
			"/usr/local/kubebuilder/bin" + " " +
			"or have to set environment variable" + " " +
			"KUBEBUILDER_ASSETS to the path these binaries reside in")
		t.Error(err)
	}

	// Create manager
	mgrOpt := manager.Options{}
	mgr, err := manager.New(restConf, mgrOpt)
	if err != nil {
		t.Error(err)
	}

	ctx := context.TODO()
	go func() {
		_ = mgr.GetCache().Start(ctx)
	}()

	// Set up kubectl command
	if envPath := os.Getenv("KUBEBUILDER_ASSETS"); envPath != "" {
		kubectlPath = filepath.Join(envPath, "kubectl")
	} else {
		kubectlPath = filepath.Join("/usr/local/kubebuilder/bin", "kubectl")
	}

	// kubectl arg for "kubectl apply"
	applyArgs := []string{"apply"}
	applyArgs = append(applyArgs, "--server="+restConf.Host)

	// kubectl arg for "kubectl delete"
	deleteArgs := []string{"delete"}
	deleteArgs = append(deleteArgs, "--server="+restConf.Host)

	// Configure globalObjectTracker
	globalObjectTracker.mgr = mgr

	testCases := []struct {
		subtest          string
		metricsDuration  int
		actions          []string
		defaultNamespace string
		objects          [][]string
		wants            []string
	}{
		// It's better to use different kind of K8s object for each test cases
		{
			subtest:          "Create K8s object",
			metricsDuration:  0,
			actions:          []string{"Create"},
			defaultNamespace: "",
			objects: [][]string{
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns1\n",
				},
			},
			wants: []string{
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns1", namespace = ""} 1
				`,
			},
		},
		{
			subtest:          "Update K8s object",
			metricsDuration:  0,
			actions:          []string{"Create", "Update"},
			defaultNamespace: "",
			objects: [][]string{
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns2\n",
					"kind: Role\n" +
						"apiVersion: rbac.authorization.k8s.io/v1\n" +
						"metadata:\n" +
						"   name: r2\n" +
						"   namespace: ns2\n" +
						"rules:\n" +
						`   - apiGroups: [""]` + "\n" +
						`     resources: ["pods"]` + "\n" +
						`     verbs: ["get"]`,
				},
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns2\n",
					"kind: Role\n" +
						"apiVersion: rbac.authorization.k8s.io/v1\n" +
						"metadata:\n" +
						"   name: r2\n" +
						"   namespace: ns2\n" +
						"rules:\n" +
						`   - apiGroups: [""]` + "\n" +
						`     resources: ["pods"]` + "\n" +
						`     verbs: ["get", "list"]`,
				},
			},
			wants: []string{
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns2", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "rbac.authorization.k8s.io/v1/Role", name = "r2", namespace = "ns2"} 1
				`,
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns2", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "rbac.authorization.k8s.io/v1/Role", name = "r2", namespace = "ns2"} 1
				`,
			},
		},
		{
			subtest:          "Delete K8s object",
			metricsDuration:  0,
			actions:          []string{"Create", "Delete"},
			defaultNamespace: "ns3",
			objects: [][]string{
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns3\n",
					"kind: Secret\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: s3\n" +
						"type: Opaque\n" +
						"data:\n" +
						"   name: dGVzdA==\n",
				},
				{
					"kind: Secret\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: s3\n" +
						"type: Opaque\n" +
						"data:\n" +
						"   name: dGVzdA==\n",
				},
			},
			wants: []string{
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns3", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Secret", name = "s3", namespace = "ns3"} 1
				`,
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns3", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Secret", name = "s3", namespace = "ns3"} 0
				`,
			},
		},
		{
			subtest:          "Delete metrics after specified duration(duration=2)",
			metricsDuration:  2,
			actions:          []string{"Create", "Delete", "Create", "Create"},
			defaultNamespace: "",
			objects: [][]string{
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns4\n",
					"kind: Secret\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: s4\n" +
						"   namespace: ns4\n" +
						"type: Opaque\n" +
						"data:\n" +
						"   name: dGVzdA==\n",
				},
				{
					"kind: Secret\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: s4\n" +
						"   namespace: ns4\n" +
						"type: Opaque\n" +
						"data:\n" +
						"   name: dGVzdA==\n",
				},
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns4\n",
				},
				{
					"kind: Namespace\n" +
						"apiVersion: v1\n" +
						"metadata:\n" +
						"   name: ns4\n",
				},
			},
			wants: []string{
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns4", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Secret", name = "s4", namespace = "ns4"} 1
				`,
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns4", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Secret", name = "s4", namespace = "ns4"} 0
				`,
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns4", namespace = ""} 1
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Secret", name = "s4", namespace = "ns4"} 0
				`,
				`
				# HELP declarative_reconciler_managed_objects_record Track the number of objects in manifest
				# TYPE declarative_reconciler_managed_objects_record gauge
				declarative_reconciler_managed_objects_record {group_version_kind = "v1/Namespace", name = "ns4", namespace = ""} 1
				`,
			},
		},
	}

	for _, st := range testCases {
		t.Run(st.subtest, func(t *testing.T) {
			globalObjectTracker.SetMetricsDuration(st.metricsDuration)

			for i, yobjList := range st.objects {
				var cmd *exec.Cmd
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				var cmdArgs []string

				var yobj string
				var jobjList = [][]byte{}
				var objList = []*manifest.Object{}

				for i, yitem := range yobjList {
					if i == 0 {
						yobj = yitem
					} else {
						yobj = yobj + "---\n" + yitem
					}
				}

				// YAML to JSON
				for _, yitem := range yobjList {
					jobj, err := yaml.YAMLToJSON([]byte(yitem))
					if err != nil {
						t.Error(err)
					}
					jobjList = append(jobjList, jobj)
				}

				// JSON to manifest.Object
				for _, jobj := range jobjList {
					mobj, err := manifest.ParseJSONToObject(jobj)
					if err != nil {
						t.Error(err)
					}
					objList = append(objList, mobj)
				}

				// Run addIfNotPresent
				err = globalObjectTracker.addIfNotPresent(objList, st.defaultNamespace)
				if err != nil {
					t.Error(err)
				}

				// Set up kubectl command
				if st.actions[i] != "Delete" {
					if len(st.defaultNamespace) != 0 {
						cmdArgs = append(applyArgs, "-n", st.defaultNamespace, "-f", "-")
					} else {
						cmdArgs = append(applyArgs, "-f", "-")
					}
				} else {
					if len(st.defaultNamespace) != 0 {
						cmdArgs = append(deleteArgs, "-n", st.defaultNamespace, "-f", "-")
					} else {
						cmdArgs = append(deleteArgs, "-f", "-")
					}
				}
				cmd = exec.Command(kubectlPath, cmdArgs...)
				cmd.Stdin = strings.NewReader(yobj)
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr

				if err := cmd.Run(); err != nil {
					t.Logf("action: %v\n", st.actions[i])
					t.Logf("stdout: %v\n", stdout.String())
					t.Logf("stderr: %v\n", stderr.String())
					t.Error(err)
				}

				// Wait for reflector sees K8s object change in K8s API server & adds it to DeltaFIFO
				// then controller pops it and eventhandler updates metrics
				// If we ommit it, there is a chance call of testutil.CollectAndCompare is too fast & fails.
				_ = mgr.GetCache().WaitForCacheSync(ctx)
				time.Sleep(time.Second * 10)

				// Check for metrics
				err = testutil.CollectAndCompare(managedObjectsRecord, strings.NewReader(st.wants[i]))
				if err != nil {
					t.Logf("No. of action in subtest: %v\n", i)
					t.Error(err)
				}
			}

		})

		managedObjectsRecord.Reset()
	}
}
