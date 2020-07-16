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
	infos, err := res.Infos()
	if err != nil {
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

	return applyOpts.Run()
}
