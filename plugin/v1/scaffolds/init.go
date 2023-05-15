/*
Copyright 2023 The Kubernetes Authors.

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

package scaffolds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kubebuilder/v3/pkg/plugin"
	"sigs.k8s.io/kubebuilder/v3/pkg/plugin/external"
	"sigs.k8s.io/kubebuilder/v3/pkg/plugin/util"
)

var InitFlags = []external.Flag{}

var InitMeta = plugin.SubcommandMetadata{
	Description: "Initialize a new project with declarative plugin",
	Examples: `
	Scaffold with go/v4:
	$ kubebuilder init --plugins go/v4,declarative/v1
	`,
}

// InitCmd handles all the logic for the `init` subcommand
func InitCmd(pr *external.PluginRequest) external.PluginResponse {
	pluginResponse := external.PluginResponse{
		APIVersion: "v1",
		Command:    "init",
		Universe:   pr.Universe,
	}

	if err := updateDockerfile(); err != nil {
		pluginResponse.Error = true
		pluginResponse.ErrorMsgs = append(pluginResponse.ErrorMsgs, err.Error())
	}

	if content, err := os.ReadFile("Dockerfile"); err != nil {
		pluginResponse.Error = true
		pluginResponse.ErrorMsgs = append(pluginResponse.ErrorMsgs, err.Error())

	} else {
		pluginResponse.Universe["Dockerfile"] = string(content)
	}

	return pluginResponse
}

// updateDockerfile will add channels staging required for declarative plugin
func updateDockerfile() error {
	// fmt.Println("updating Dockerfile to add channels/ directory in the image")
	dockerfile := filepath.Join("Dockerfile")
	controllerPath := "internal/controller/"

	// nolint:lll
	err := insertCodeIfDoesNotExist(dockerfile,
		fmt.Sprintf("COPY %s %s", controllerPath, controllerPath),
		"\n# https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/blob/master/docs/addon/walkthrough/README.md#adding-a-manifest\n# Stage channels and make readable\nCOPY channels/ /channels/\nRUN chmod -R a+rx /channels/")
	if err != nil {
		return err
	}

	err = insertCodeIfDoesNotExist(dockerfile,
		"COPY --from=builder /workspace/manager .",
		"\n# copy channels\nCOPY --from=builder /channels /channels\n")
	if err != nil {
		return err
	}

	return nil
}

// insertCodeIfDoesNotExist insert code if it does not already exists
func insertCodeIfDoesNotExist(filename, target, code string) error {
	// false positive
	// nolint:gosec
	contents, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	idx := strings.Index(string(contents), code)
	if idx != -1 {
		return nil
	}

	return util.InsertCode(filename, target, code)
}
