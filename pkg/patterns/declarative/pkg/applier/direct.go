package applier

import (
	"context"
	//"io/ioutil"
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
	a           apply.ApplyOptions
	ConfigFlags genericclioptions.RESTClientGetter
}

func NewDirectApplier() *DirectApplier {
	return &DirectApplier{
		ConfigFlags: genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag(),
	}
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
	restClient := d.ConfigFlags
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

//func (d *DirectApplier) Apply(ctx context.Context,
//	namespace string,
//	manifest string,
//	validate bool,
//	extraArgs ...string,
//) error {
//
//	tmpFile, err := ioutil.TempFile("", "tmp-manifest-*.yaml")
//	if err != nil {
//		return err
//	}
//	tmpFile.Write([]byte(manifest))
//	tmpFile.Close()
//	defer os.Remove(tmpFile.Name())
//	ioStreams := genericclioptions.IOStreams{
//		In:     tmpFile,
//		Out:    os.Stdout,
//		ErrOut: os.Stderr,
//	}
//	restClient := d.ConfigFlags
//	f := cmdutil.NewFactory(restClient)
//	schema, err := f.Validator(validate)
//	if err != nil {
//		return err
//	}
//	applyOpts := apply.NewApplyOptions(ioStreams)
//
//	applyOpts.DynamicClient, err = f.DynamicClient()
//	if err != nil {
//		return err
//	}
//	applyOpts.DeleteOptions, _ = applyOpts.DeleteFlags.ToOptions(applyOpts.DynamicClient, applyOpts.IOStreams)
//
//	applyOpts.Namespace, applyOpts.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
//	if namespace != "" {
//		applyOpts.Namespace = namespace
//		//applyOpts.EnforceNamespace = true
//	}
//	applyOpts.Validator = schema
//	applyOpts.Builder = f.NewBuilder()
//	applyOpts.Mapper, err = f.ToRESTMapper()
//	applyOpts.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
//		applyOpts.PrintFlags.NamePrintFlags.Operation = operation
//		cmdutil.PrintFlagsWithDryRunStrategy(applyOpts.PrintFlags, applyOpts.DryRunStrategy)
//		return applyOpts.PrintFlags.ToPrinter()
//	}
//	applyOpts.DeleteOptions = &cmdDelete.DeleteOptions{
//		IOStreams: ioStreams,
//	}
//	applyOpts.DeleteOptions.Filenames = []string{tmpFile.Name()}
//
//	return applyOpts.Run()
//}
