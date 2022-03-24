package applier

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DirectApplier struct {
}

var _ Applier = &DirectApplier{}

func NewDirectApplier() *DirectApplier {
	return &DirectApplier{}
}

func (d *DirectApplier) Apply(ctx context.Context, opt ApplierOptions) error {
	log := log.Log
	ioReader := strings.NewReader(opt.Manifest)

	buffer := &bytes.Buffer{}

	ioStreams := genericclioptions.IOStreams{
		In:     bytes.NewReader(nil),
		Out:    buffer,
		ErrOut: buffer,
	}

	restClientGetter := &staticRESTClientGetter{
		RESTMapper: opt.RESTMapper,
		RESTConfig: opt.RESTConfig,
	}

	args := append(opt.ExtraArgs, "-f", "-")
	if !opt.Validate {
		args = append(args, "--validate=false")
	}

	log.V(4).Info("applying manifests", "args", args, "manifests", opt.Manifest)

	cmd, o := NewCmdApply(ioStreams)
	if err := cmd.ParseFlags(args); err != nil {
		return fmt.Errorf("parse kubectl apply args failed, args: %v, errors: %s", args, err)
	}
	if err := o.Complete(cmdutil.NewFactory(restClientGetter), cmd); err != nil {
		return fmt.Errorf("apply manifests failed, args: %v, errors: %s", args, err)
	}
	res := o.Builder.Unstructured().Stream(ioReader, "manifestString").Do()
	if infos, err := res.Infos(); err != nil {
		return err
	} else {
		o.SetObjects(infos)
	}

	if err := o.Run(); err != nil {
		return fmt.Errorf("apply manifests failed, args: %v, errors: %s, msg:%s", args, err, buffer.String())
	}

	log.Info("applying manifest succeed", "message", buffer.String())
	return nil
}

// staticRESTClientGetter returns a fixed RESTClient
type staticRESTClientGetter struct {
	RESTConfig      *rest.Config
	DiscoveryClient discovery.CachedDiscoveryInterface
	RESTMapper      meta.RESTMapper
	namespace       string
}

var _ resource.RESTClientGetter = &staticRESTClientGetter{}

func (s *staticRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	if s.RESTConfig == nil {
		return nil, fmt.Errorf("RESTConfig not set")
	}
	return s.RESTConfig, nil
}
func (s *staticRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	if s.DiscoveryClient == nil {
		return nil, fmt.Errorf("DiscoveryClient not set")
	}
	return s.DiscoveryClient, nil
}
func (s *staticRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	if s.RESTMapper == nil {
		return nil, fmt.Errorf("RESTMapper not set")
	}
	return s.RESTMapper, nil
}
func (s *staticRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return namespaceStub(s.namespace)
}

type namespaceStub string

func (n namespaceStub) Namespace() (string, bool, error) {
	return string(n), false, nil
}

// below methods should never be called
func (namespaceStub) RawConfig() (api.Config, error) {
	panic("implement me")
}

func (namespaceStub) ClientConfig() (*rest.Config, error) {
	panic("implement me")
}

func (namespaceStub) ConfigAccess() clientcmd.ConfigAccess {
	panic("implement me")
}

func NewCmdApply(ioStreams genericclioptions.IOStreams) (*cobra.Command, *apply.ApplyOptions) {
	o := apply.NewApplyOptions(ioStreams)

	// Store baseName for use in printing warnings / messages involving the base command name.
	// This is useful for downstream command that wrap this one.

	cmd := &cobra.Command{}

	// bind flag structs
	o.DeleteFlags.AddFlags(cmd)
	o.RecordFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.Overwrite, "overwrite", o.Overwrite, "Automatically resolve conflicts between the modified and live configuration by using values from the modified configuration")
	cmd.Flags().BoolVar(&o.Prune, "prune", o.Prune, "Automatically delete resource objects, including the uninitialized ones, that do not appear in the configs and are created by either apply or create --save-config. Should be used with either -l or --all.")
	cmdutil.AddValidateFlags(cmd)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "Select all resources in the namespace of the specified resource types.")
	cmd.Flags().StringArrayVar(&o.PruneWhitelist, "prune-whitelist", o.PruneWhitelist, "Overwrite the default whitelist with <group/version/kind> for --prune")
	cmd.Flags().BoolVar(&o.OpenAPIPatch, "openapi-patch", o.OpenAPIPatch, "If true, use openapi to calculate diff when the openapi presents and the resource can be found in the openapi spec. Otherwise, fall back to use baked-in types.")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddServerSideApplyFlags(cmd)
	cmdutil.AddFieldManagerFlagVar(cmd, &o.FieldManager, apply.FieldManagerClientSideApply)

	return cmd, o
}
