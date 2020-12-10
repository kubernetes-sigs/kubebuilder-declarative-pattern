package applier

import (
	"context"
	"os"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdDelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type DirectApplier struct {
	a apply.ApplyOptions
}

func NewDirectApplier() *DirectApplier {
	return &DirectApplier{}
}

func (d *DirectApplier) Apply(ctx context.Context,
	namespace string,
	manifest string,
	validate bool,
	extraArgs ...string,
) error {
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	restClient := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	ioReader := strings.NewReader(manifest)

	b := resource.NewBuilder(restClient)
	res := b.Unstructured().Stream(ioReader, "manifestString").Do()
	// res.Infos returns errors for types not on the currently-connected Client.
	// If a manifest includes both a CRD and a CR, and the CRD is not already on
	// the cluster when Apply() is called, this will return an error and a partial
	// set of objects to apply. We should attempt to apply this partial set even
	// if err is non-nil.
	infos, err := res.Infos()
	if infos == nil {
		return err
	}

	applyOpts := apply.NewApplyOptions(ioStreams)
	applyOpts.Namespace = namespace
	applyOpts.SetObjects(infos)
	applyOpts.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		applyOpts.PrintFlags.NamePrintFlags.Operation = operation
		cmdutil.PrintFlagsWithDryRunStrategy(applyOpts.PrintFlags, applyOpts.DryRunStrategy)
		return applyOpts.PrintFlags.ToPrinter()
	}
	applyOpts.DeleteOptions = &cmdDelete.DeleteOptions{
		IOStreams: ioStreams,
	}

	// We can only return one error here, so prefer returning the error
	// encountered while applying the resources over the error parsing the
	// objects.
	if err := applyOpts.Run(); err != nil {
		return err
	}
	return err
}
