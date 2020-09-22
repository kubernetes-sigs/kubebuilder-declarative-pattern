## Reconciler Options in Kubebuilder-declarative-pattern

Kubebuilder-declarative-pattern is structured in a way that makes it easy for you to turn functionality(provided in kubebuilder-declarative-patter) on and off in the operator you have created. This also makes it easy to add new functionality to your operator. This README serves as a references for these functionalities and indicates which ones are enabled by default.

The options are:
## WithRawManifestOperation
WithRawManifestOperation takes in a set of functions that transforms raw string manifests before applying it.
The functions should be of the form:
```
type ManifestOperation = func(context.Context, DeclarativeObject, string) (string, error)
```

## WithObjectTransform
WithObjectTransform takes in a set of functions that transforms the manifest objects before applying it
The functions should be of the form:
```
type ObjectTransform = func(context.Context, DeclarativeObject, *manifest.Objects) error
```

## WithManifestController
WithManifestController overrides the default source for loading manifests.

## WithApplyPrune
WithApplyPrune turns on the --prune behavior of kubectl apply. This behavior deletes any objects that exist in the API server that are not deployed by the current version of the manifest which match a label specific to the addon instance.
This option requires (WithLabels)[#withLabels] to be used.

## WithOwner
WithOwner sets an owner ref on each deployed object by the (OwnerSelector)[https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/blob/master/pkg/patterns/declarative/options.go#L74].

## WithLabels
WithLabels sets a fixed set of labels configured provided by a LabelMaker to all deployment objecs for a given DeclarativeObject

## WithStatus
WithStatus provides a (Status)[https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/blob/master/pkg/patterns/declarative/status.go#L26] interface that will be used during Reconcile.

## WithPreserveNamespace
WithPreserveNamespace preserves the namespaces defined in the deployment manifest
instead of matching the namespace of the DeclarativeObject

## WithApplyKustomize
WithApplyKustomize run kustomize build to create final manifest

## WithManagedApplication
WithManagedApplication is a transform that will modify the Application object in the deployment to match the configuration of the rest of the deployment.

## WithApplyValidation
WithApplyValidation enables validation with kubectl apply

## WithReconcileMetrics
WithReconcileMetrics enables metrics of declarative reconciler.
