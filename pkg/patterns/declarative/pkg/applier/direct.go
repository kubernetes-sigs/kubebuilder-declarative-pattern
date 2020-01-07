package applier

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdDelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type DirectApplier struct {
	restConfig *rest.Config
	a          apply.ApplyOptions
}

func NewDirectApplier() *DirectApplier {
	return NewDirectApplierWithRESTConfig(nil)
}

func NewDirectApplierWithRESTConfig(restConfig *rest.Config) *DirectApplier {
	return &DirectApplier{
		restConfig: restConfig,
	}
}

func (d *DirectApplier) Apply(ctx context.Context,
	namespace string,
	objects *manifest.Objects,
	validate bool,
	extraArgs ...string,
) error {
	var restClientGetter resource.RESTClientGetter
	if d.restConfig == nil {
		restClientGetter = genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	} else {
		restClientGetter = &simpleRESTClientGetter{restConfig: d.restConfig}
	}

	var crds manifest.Objects
	var others manifest.Objects

	// We need to apply CRDs first, otherwise the validation done as part of apply will fail
	for _, obj := range objects.Items {
		if obj.Kind == "CustomResourceDefinition" {
			crds.Items = append(crds.Items, obj)
			continue
		}
		others.Items = append(others.Items, obj)
	}

	if err := d.applyObjects(ctx, namespace, restClientGetter, &crds); err != nil {
		return err
	}
	if err := d.applyObjects(ctx, namespace, restClientGetter, &others); err != nil {
		return err
	}
	return nil
}

func (d *DirectApplier) applyObjects(ctx context.Context,
	namespace string,
	restClientGetter resource.RESTClientGetter,
	objects *manifest.Objects,
) error {
	if len(objects.Items) == 0 {
		return nil
	}

	json, err := objects.JSONManifest()
	if err != nil {
		return err
	}

	ioReader := strings.NewReader(json)
	b := resource.NewBuilder(restClientGetter)
	res := b.Unstructured().Stream(ioReader, "manifestString").Do()
	infos, err := res.Infos()
	if err != nil {
		return fmt.Errorf("error parsing manifest: %w", err)
	}

	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
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

type simpleRESTClientGetter struct {
	restConfig *rest.Config
}

// ToRESTConfig implements RESTClientGet
func (f *simpleRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return f.restConfig, nil
}

// ToDiscoveryClient implements RESTClientGet
func (f *simpleRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	client, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(client), nil
}

// ToRESTMapper implements RESTClientGet
func (f *simpleRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	//expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return mapper, nil
}
