/*
Copyright 2018 The Kubernetes Authors.

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

package kubectlcmd

import (
	"bytes"
	"context"
	"fmt"
	"k8s.io/cli-runtime/pkg/printers"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// New creates a Client that runs kubectl avaliable on the path with default authentication
func New() *Client {
	return &Client{cmdSite: &console{}}
}

// Client provides an interface to kubectl
type Client struct {
	cmdSite commandSite
}

// commandSite allows for tests to mock cmd.Run() events
type commandSite interface {
	Run(*exec.Cmd) error
}
type console struct {
}

func (console) Run(c *exec.Cmd) error {
	return c.Run()
}

// Apply runs the kubectl apply with the provided manifest argument
func (c *Client) ApplyAlt(ctx context.Context, namespace string, manifest string, validate bool,
	extraArgs ...string) error {
	log := log.Log

	log.Info("applying manifest")

	args := []string{"apply"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Not doing --validate avoids downloading the OpenAPI
	// which can save a lot work & memory
	args = append(args, "--validate="+strconv.FormatBool(validate))

	args = append(args, extraArgs...)
	args = append(args, "-f", "-")

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(manifest)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.WithValues("command", "kubectl").WithValues("args", args).Info("executing kubectl")

	err := c.cmdSite.Run(cmd)
	if err != nil {
		log.WithValues("stdout", stdout.String()).WithValues("stderr", stderr.String()).Error(err, "error from running kubectl apply")
		log.Info(fmt.Sprintf("manifest:\n%v", manifest))
		return fmt.Errorf("error from running  apply: %v", err)
	}

	log.WithValues("stdout", stdout.String()).WithValues("stderr", stderr.String()).V(2).Info("ran kubectl apply")

	return nil
}

func(c *Client) Apply(ctx context.Context, namespace string, manifest string, validate bool,
	extraArgs ...string) error{
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	restClient := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	//f := util.NewFactory(restClient)
	ioReader := strings.NewReader(manifest)

	b := resource.NewBuilder(restClient)
	res := b.Unstructured().Stream(ioReader, "cm").Do()
	//infos = append(infos, namespaceObj)
	infos, err := res.Infos()
	if err != nil {
		return err
	}
	applyOpts := apply.NewApplyOptions(ioStreams)
	applyOpts.Namespace = namespace
	buildApplyOpts(applyOpts, infos, ioStreams)
	err = applyOpts.Run()
	return err
}

func buildApplyOpts (a *apply.ApplyOptions, infos []*resource.Info,
	ioStreams genericclioptions.IOStreams) {
	a.SetObjects(infos)
	a.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		a.PrintFlags.NamePrintFlags.Operation = operation
		cmdutil.PrintFlagsWithDryRunStrategy(a.PrintFlags, a.DryRunStrategy)
		return a.PrintFlags.ToPrinter()
	}
	a.DeleteOptions = &delete.DeleteOptions{
		IOStreams:           ioStreams,
	}
}
