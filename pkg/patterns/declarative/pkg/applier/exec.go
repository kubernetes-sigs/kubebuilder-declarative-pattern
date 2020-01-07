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

package applier

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// NewExec creates a ExecKubectl that runs kubectl avaliable on the path with default authentication
func NewExec() *ExecKubectl {
	return NewExecWithRESTConfig(nil)
}

// NewExecWithRESTConfig creates a ExecKubectl that runs kubectl avaliable on the path with specified configuration
func NewExecWithRESTConfig(restConfig *rest.Config) *ExecKubectl {
	return &ExecKubectl{
		restConfig: restConfig,
		cmdSite:    &console{},
	}
}

// ExecKubectl provides an interface to kubectl
type ExecKubectl struct {
	restConfig *rest.Config
	cmdSite    commandSite
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

func buildKubeconfig(restConfig *rest.Config) ([]byte, error) {
	clientConfig := clientcmdapi.Config{}
	{
		context := &clientcmdapi.Context{
			Cluster:  "target",
			AuthInfo: "target",
		}

		authInfo := &clientcmdapi.AuthInfo{
			ClientCertificateData: restConfig.CertData,
			ClientKeyData:         restConfig.KeyData,
			Token:                 restConfig.BearerToken,
		}

		cluster := &clientcmdapi.Cluster{
			Server:                   restConfig.Host,
			CertificateAuthorityData: restConfig.CAData,
		}

		clientConfig.CurrentContext = "target"
		clientConfig.Contexts = map[string]*clientcmdapi.Context{"target": context}
		clientConfig.AuthInfos = map[string]*clientcmdapi.AuthInfo{"target": authInfo}
		clientConfig.Clusters = map[string]*clientcmdapi.Cluster{"target": cluster}
	}

	b, err := clientcmd.Write(clientConfig)
	if err != nil {
		return nil, xerrors.Errorf("error building kubeconfig: %w", err)
	}

	return b, nil
}

// Apply runs the kubectl apply with the provided manifest argument
func (c *ExecKubectl) Apply(ctx context.Context, namespace string, objects *manifest.Objects, validate bool,
	extraArgs ...string) error {
	log := log.Log

	json, err := objects.JSONManifest()
	if err != nil {
		return err
	}

	args := []string{"apply"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Not doing --validate avoids downloading the OpenAPI
	// which can save a lot work & memory
	args = append(args, "--validate="+strconv.FormatBool(validate))

	if c.restConfig != nil {
		kubeconfig, err := buildKubeconfig(c.restConfig)
		if err != nil {
			return xerrors.Errorf("error building kubeconfig: %w", err)
		}

		f, err := ioutil.TempFile("", "kubeconfig")
		if err != nil {
			return xerrors.Errorf("error creating temp file: %w", err)
		}

		defer func() {
			if err := os.Remove(f.Name()); err != nil {
				klog.Errorf("error removing kubeconfig temp file %s: %v", f.Name(), err)
			}
		}()

		if _, err := f.Write(kubeconfig); err != nil {
			return xerrors.Errorf("error writing kubeconfig: %w", err)
		}

		if err := f.Close(); err != nil {
			return xerrors.Errorf("error writing kubeconfig: %w", err)
		}

		args = append(args, "--kubeconfig", f.Name())
	}

	args = append(args, extraArgs...)
	args = append(args, "-f", "-")

	log.Info("applying manifest with kubectl %s", strings.Join(args, " "))

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(json)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.WithValues("command", "kubectl").WithValues("args", args).Info("executing kubectl")

	if err := c.cmdSite.Run(cmd); err != nil {
		log.WithValues("stdout", stdout.String()).WithValues("stderr", stderr.String()).Error(err, "error from running kubectl apply")
		log.Info(fmt.Sprintf("manifest:\n%v", json))
		return fmt.Errorf("error from running kubectl apply: %v", err)
	}

	log.WithValues("stdout", stdout.String()).WithValues("stderr", stderr.String()).V(2).Info("ran kubectl apply")

	return nil
}
