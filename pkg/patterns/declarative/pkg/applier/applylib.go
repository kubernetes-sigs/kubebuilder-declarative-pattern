package applier

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"sigs.k8s.io/kubebuilder-declarative-pattern/applylib/applyset"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type ApplySetApplier struct {
	patchOptions metav1.PatchOptions
}

var _ Applier = &ApplySetApplier{}

func NewApplySetApplier(patchOptions metav1.PatchOptions) *ApplySetApplier {
	return &ApplySetApplier{patchOptions: patchOptions}
}

func (a *ApplySetApplier) Apply(ctx context.Context, opt ApplierOptions) error {
	objects, err := manifest.ParseObjects(ctx, opt.Manifest)
	if err != nil {
		return fmt.Errorf("error parsing manifest: %w", err)
	}

	for _, arg := range opt.ExtraArgs {
		switch arg {

		default:
			return fmt.Errorf("extraArg %q is not supported by the ApplySetApplier", arg)
		}
	}

	dynamicClient, err := dynamic.NewForConfig(opt.RESTConfig)
	if err != nil {
		return fmt.Errorf("error building dynamic client: %w", err)
	}

	restMapper := opt.RESTMapper

	options := applyset.Options{
		PatchOptions: a.patchOptions,
		RESTMapper:   restMapper,
		Client:       dynamicClient,
	}
	s, err := applyset.New(options)
	if err != nil {
		return fmt.Errorf("error creating applyset: %w", err)
	}

	// Populate the namespace on any namespace-scoped objects
	if opt.Namespace != "" {
		// for _, obj := range objects.Items {
		// 	if obj.GetNamespace() != "" {
		// 		obj.SetNamespace(opt.Namespace)
		// 	}
		// }
		return fmt.Errorf("namespace override not (yet) supported on ApplySetapplier")
	}

	var applyableObjects []applyset.ApplyableObject
	for _, obj := range objects.Items {
		applyableObject := obj.UnstructuredObject()
		applyableObjects = append(applyableObjects, applyableObject)
	}
	if err := s.SetDesiredObjects(applyableObjects); err != nil {
		return fmt.Errorf("error setting desired objects for apply: %w", err)
	}

	results, err := s.ApplyOnce(ctx)
	if err != nil {
		// TODO: Aggregate errors?
		return fmt.Errorf("error applying objects: %w", err)
	}
	if !results.AllApplied() {
		return fmt.Errorf("not all objects applied")
	}

	// TODO: Check healthy

	return nil
}
