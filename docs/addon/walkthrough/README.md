## Walkthrough: Creating a new Operator

This walkthrough is for creating an operator to run the kubernetes dashboard.

### Basics

Install the following depenencies:

- [kubebuilder](https://book.kubebuilder.io/quick-start.html#installation) (tested with 2.0.0-alpha.4)
- [kustomize](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md)
- docker
- kubectl
- golang (>=1.11 for go modules)

Create a new directory and use kubebuilder to scafold the operator:

```
export GO111MODULE=on
mkdir -p dashboard-operator/
cd dashboard-operator/
kubebuilder init --domain example.org --license apache2 --owner "TODO($USER): assign copyright" 
```

Add the patterns to your project:

```
go get sigs.k8s.io/kubebuilder-declarative-pattern
```

### Adding our first CRD

```
# generate the API/controllers
kubebuilder create api --controller=true --example=false --group=addons --kind=Dashboard --make=false --namespaced=true --resource=true --version=v1alpha1
# remove the  test suites that are more checking that kubebuilder is working
find . -name "*_test.go" -delete
```


This creates API type definitions under `api/v1alpha1/`, and a basic
controller under `controllers/`

* Generate code: `make generate`

* You should now be able to `go run main.go` (or `make run`),
  though it will exit with an error from being unable to find the dashboard CRD.

### Adding a manifest

The addon operator pattern is based on declarative manifests; the framework is
able to load the manifests and apply them. Today we exec `kubectl apply`, but
when [server-side-apply](https://github.com/kubernetes/enhancements/issues/555) 
is available we'll use that.  

We suggest that even advanced operators use a manifest for their core objects.
It's always possible to manipulate the manifest before applying them (eg, adding labels,
changing namespaces, and tweaking flags)

Some other advantages:

* Working with manifests lets us release a new dashboard version without needing
  a new operator version
* The declarative manifest makes it easier for users to understand what is
  changing in each version
* It should result in less / simpler code

For now, we embed the manifests into the image, but we'll be evolving this, for example sourcing manifests from a bundle or over https.

Create a manifest under `channels/packages/<packagename>/<version>/manifest.yaml`

```bash
mkdir -p channels/packages/dashboard/1.8.3/
wget -O channels/packages/dashboard/1.8.3/manifest.yaml https://raw.githubusercontent.com/kubernetes/dashboard/v1.8.3/src/deploy/recommended/kubernetes-dashboard.yaml
```

We have a notion of "channels", which is a stream of updates.  We'll have
settings to automatically update or prompt-for-update when the channel updates.
Currently if you don't specify a channel in your CRD, you get the version
currently in the stable channel.

We need to define the default stable channel, so create `channels/stable`:

```bash
cat > channels/stable <<EOF
manifests:
- name: dashboard
  version: 1.8.3
EOF
```

### Adding the framework into our types

We now want to plug the framework into our types, so that our CRs will look like
other addon operators.

We begin by editing the api type, we add some common fields.  The idea is that
CommonSpec and CommonStatus form a common contract that we expect all addons to
support (and we can hopefully evolve the contract with just a recompile!)

Modify `api/v1alpha1/dashboard_types.go` to:

* add an import for `addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"`
* add a field `addonv1alpha1.CommonSpec` to the Spec object
* add a field `addonv1alpha1.CommonStatus` to the Status object
* add the accessor functions (ComponentName, CommonSpec, ..)

Your `dashboard_types.go` file should now additionally contain:

```go
import addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"

type DashboardSpec struct {
	addonv1alpha1.CommonSpec `json:",inline"`
}

type DashboardStatus struct {
	addonv1alpha1.CommonStatus `json:",inline"`
}

var _ addonv1alpha1.CommonObject = &Dashboard{}

func (c *Dashboard) ComponentName() string {
	return "dashboard"
}

func (c *Dashboard) CommonSpec() addonv1alpha1.CommonSpec {
	return c.Spec.CommonSpec
}

func (c *Dashboard) GetCommonStatus() addonv1alpha1.CommonStatus {
	return c.Status.CommonStatus
}

func (c *Dashboard) SetCommonStatus(s addonv1alpha1.CommonStatus) {
	c.Status.CommonStatus = s
}

```

### Using the framework in the controller

We replace the controller code `controllers/dashboard_controller.go`:

We are delegating most of the logic to `declarative.Reconciler`

```go
package controllers

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

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/status"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "dashboard-operator/api/v1alpha1"
)

var _ reconcile.Reconciler = &DashboardReconciler{}

// DashboardReconciler reconciles a Dashboard object
type DashboardReconciler struct {
	declarative.Reconciler
	client.Client
	Log logr.Logger

	watchLabels declarative.LabelMaker
}

func (r *DashboardReconciler) setupReconciler(mgr ctrl.Manager) error {
	labels := map[string]string{
		"k8s-app": "kubernetes-dashboard",
	}

	r.watchLabels = declarative.SourceLabel(mgr.GetScheme())

	return r.Reconciler.Init(mgr, &api.Dashboard{},
		declarative.WithObjectTransform(declarative.AddLabels(labels)),
		declarative.WithOwner(declarative.SourceAsOwner),
		declarative.WithLabels(r.watchLabels),
		declarative.WithStatus(status.NewBasic(mgr.GetClient())),
		declarative.WithPreserveNamespace(),
		declarative.WithApplyPrune(),
	)
}

// +kubebuilder:rbac:groups=addons.example.org,resources=dashboards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=addons.example.org,resources=dashboards/status,verbs=get;update;patch
func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.setupReconciler(mgr); err != nil {
		return err
	}

	c, err := controller.New("dashboard-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Dashboard
	err = c.Watch(&source.Kind{Type: &api.Dashboard{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to deployed objects
	_, err = declarative.WatchAll(mgr.GetConfig(), c, r, r.watchLabels)
	if err != nil {
		return err
	}

	return nil
}

// for WithApplyPrune
// +kubebuilder:rbac:groups=*,resources=*,verbs=list

// +kubebuilder:rbac:groups=addons.example.org,resources=dashboards,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts;secrets,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;delete;patch
// RBAC roles that need to be granted:
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=create
// TODO: can be scoped to resourceNames: ["kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs"]
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;update;delete
// TODO: can be scoped to resourceNames: ["kubernetes-dashboard-settings"]
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;update;delete
// TODO: can be scoped to resourceNames: ["heapster"]
// +kubebuilder:rbac:groups="",resources=services,verbs=proxy
// TODO: can be scoped to resourceNames: ["heapster", "http:heapster:", "https:heapster:"], verbs: ["get"]
// +kubebuilder:rbac:groups="",resources=services/proxy,verbs=get
```

The important things to note here:

```go
	r.Reconciler.Init(mgr, &api.Dashboard{}, "dashboard", ...)
```

We bind the `api.Dashboard` type to the `dashboard` package in our `channels`
directory and pull in optional features of the declarative library.

Because api.Dashboard implements `addon.CommonObject` the
framework is then able to access CommonSpec and CommonStatus above, which
includes the version specifier.

### Misc

1. Add an import and init call to the top of the main() function in `main.go`:

	```go
	import (
		//..
		"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	)
	func main() {
		// after: ctrl.SetLogger(zap.Logger(true))
		addon.Init()
	}
	```

### Testing it locally

We can register the Dashboard CRD and a Dashboard object, and then try running
the controller locally.

1) We need to generate and register the CRDs:

```bash
make install
```

You can verify that the CRD is regesterd successfully:

```
kubectl get crds dashboards.addons.example.org
```

2) Create a dashboard CR:

```bash
kubectl apply -n kube-system -f config/samples/addons_v1alpha1_dashboard.yaml
```

You can verify the CR is created successfully:

```
kubectl get DashBoards -n kube-system
```

3) You should now be able to run the controller using:

`make run`

You should see your operator apply the manifest.  You can then control-C and you
should see the deployment etc that the operator has created.

e.g. `kubectl get pods -l k8s-app=kubernetes-dashboard -n kube-system` or
`kubectl get deploy kubernetes-dashboard -n kube-system`.

## Running on-cluster

Previously we were running on your machine using your kubernetes credentials.
We want to run as a Pod on the cluster for real world operator. For that, 
we'll need a Docker image and some manifests.

### Building the operator image

1. Modify the IMG value in the `Makefile` to reflect a docker registry that you 
   can write to:

```make
# Image URL to use all building/pushing image targets
IMG ?= gcr.io/<my-cool-project>/dashboard-operator:latest
```

1. Create a patch to modify the memory limit for the operator:

```bash
echo << EOF > config/default/manager_resource_patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
spec:
  template:
    spec:
      containers:
      - name: manager
        resources:
          limits:
            cpu: 100m
            memory: 150Mi
          requests:
            cpu: 100m
            memory: 20Mi
EOF
```

1. Reference the patch by adding `manager_resource_patch.yaml` to the `patches` section of `config/default/kustomization.yaml`:

```yaml
patches:
- manager_resource_patch.yaml
# ... existing patches
```

This is requried to run kubectl in the container.

1. Modify the `Dockerfile` to pull in kubectl, the manifests (in `channels/`),
   and run in a slim container:

```Dockerfile
FROM ubuntu:latest as kubectl
RUN apt-get update
RUN apt-get install -y curl
RUN curl -fsSL https://dl.k8s.io/release/v1.13.4/bin/linux/amd64/kubectl > /usr/bin/kubectl
RUN chmod a+rx /usr/bin/kubectl

# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/dashboard-operator
COPY vendor/   vendor/
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Copy the operator and dependencies into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/dashboard-operator/manager .
COPY --from=kubectl /usr/bin/kubectl /usr/bin/kubectl
COPY channels/ channels/
ENTRYPOINT ["./manager"]
```

1. Verify everything worked by building and pushing the image:

```bash
make docker-build docker-push
```

### Generated RBAC Rules

We need a simple deployment to run our operator, and we want to run it under a
tightly-scoped RBAC role. To do that we use kubebuilder's RBAC role generation
based off of source annotations. In the future we may be able to generate RBAC
rules from the manfiest.

The RBAC rules are included in the `dashboard_controller.go` snippet you pasted above.

RBAC is the real pain-point here - we end up with a lot of permissions:
* The operator needs RBAC rules to see the CRDs.
* It needs permission to get / create / update the Deployments and other types
  that it is managing
* It needs permission to create the ClusterRoles / Roles that the dashboard
  needs
* Because of that, we also need permissions for all the permissions we are going
  to create.

The last one in particular can result in a non-trivial RBAC policy.  My approach:

* Start with minimal permissions (just watching addons.k8s.io dashboards), and
  then add permissions iteratively
* If you're going to allow list, I tend to just allow get, list and watch -
  there's not a huge security reason to treat them separately as far as I can
  see
* Similarly I treat create and patch together
* No controller should be using update (because of version skew issues), so I
  tend to grant that one begrudgingly
* The RBAC policy in the manifest may scope down the permissions even more (for
  example scoping to resourceNames), in which case we can - and should - copy
  it.  That's what we did here for dashboard.

### Installing the operator in the cluster

```bash
# install the CRD, and start the operator
make deploy
```

You can troubleshoot the operator by inspecting the controller:

```bash
kubectl -n dashboard-operator-system get deploy
kubectl -n dashboard-operator-system logs <dashboard-operator-controller-manager-pod-name> manager
```

### Create a dashboard CR

```bash
kubectl apply -n kube-system -f config/samples/addons_v1alpha1_dashboard.yaml
```

You can verify the CR is created successfully:

```
kubectl get DashBoards -n kube-system
```

You can verify that the operator has created the `kubernetes-dashboard`
deployment:

e.g. `kubectl get pods -l k8s-app=kubernetes-dashboard -n kube-system` or
`kubectl get deploy kubernetes-dashboard -n kube-system`.

## Manifest simplification: Automatic labels

Similar to how kustomize works, often you won't want labels hard-coded in the
manifest, but will use them to distinguish multiple instances.  Even if you're
writing something you expect to be a singleton instance, it can be tedious and
error-prone to specify labels on every object in the manifest.

Instead, the Reconciler can add labels to every object in the manifest:

```go
       labels := map[string]string{
               "k8s-app": "kubernetes-dashboard",
       }

       r := &ReconcileDashboard{}
       r.Reconciler.Init(mgr, &api.Dashboard{}, "dashboard",
               declarative.WithObjectTransform(declarative.AddLabels(labels)),
			   ...
	   )
```

**NOTE**: operators.AddLabels does not [_yet_](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/issues/21)
add selectors to Deployments/DaemonSets, nor to the templates.

## Manifest simplification: Automatic Namespace

The framework automatically creates objects in the same namespace as
the CR (by specifying the namespace to kubectl).  As such, we can remove the
namespaces from the manifest.

NOTE: We don't currently apply the namespace within objects.  For example, we
don't set the namespace on a RoleBinding subjects.namespace.  However, it seems
that most objects default to the same namespace - but presumably
ClusterRoleBinding will not. 

NOTE: For non-namespaces objects (ClusterRole and ClusterRoleBinding), we often
need to name them with the namespace to support multiple instances.

### Manage an Application

The framework can manage an [application](https://github.com/kubernetes-sigs/application) 
instance. The application contains human readable information in addition to deployment 
status that can be surfaced in various user interfaces.

1. Fetch the Application CRD and place it with your operators CRD:

	```bash
	curl https://raw.githubusercontent.com/kubernetes-sigs/application/master/config/crds/app_v1beta1_application.yaml -o config/crds/app_v1beta1_application.yaml
	```

1. Add an instance of the Application CR in your manifest:

	```bash
	echo <<EOF >> channels/packages/dashboard/1.8.3/manifest.yaml
	# ------------------- Application ------------------- #
	apiVersion: app.k8s.io/v1beta1
	kind: Application
	metadata:
	  name: kubernetes-dashboard
	spec:
	  descriptor:
	    type: "kubernetes-dashboard"
	    description: "Kubernetes Dashboard is a general purpose, web-based UI for Kubernetes clusters. It allows users to manage applications running in the cluster and troubleshoot them, as well as manage the cluster itself."
	    icons:
	    - src: "https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"
	      type: "image/png"
	    maintainers:
	    - name: Maintainer
	      email: maintainer@example.org
	    keywords:
	    - "addon"
	    - "dashboard"
	    links:
	    - description: Project Homepage
	      url: "https://github.com/kubernetes/dashboard"
	EOF
	```

1. Add the two options for managing the Application to your controller:

	```go
		r.Reconciler.Init(mgr, &api.Dashboard{}, "dashboard",
			...
			declarative.WithManagedApplication(r.watchLabels),
			declarative.WithObjectTransform(addon.TransformApplicationFromStatus),
			...
		)
	```

1. Rebuild the operator, reinstall the CRDs, and start the new operator. You can now see the Application:

	```bash
	kubectl -n kube-system get applications -oyaml
	```

### Next steps

* Read about [adding tests](tests.md)
* Remove cruft from the manifest yaml (Namespaces, Names, Labels)
* Explore avaliable [options](https://godoc.org/sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative)
