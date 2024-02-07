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

	"github.com/spf13/afero"
	"sigs.k8s.io/kubebuilder-declarative-pattern/plugin/v1/scaffolds/internal/templates"
	"sigs.k8s.io/kubebuilder/v3/pkg/machinery"
	"sigs.k8s.io/kubebuilder/v3/pkg/plugin"
	"sigs.k8s.io/kubebuilder/v3/pkg/plugin/external"
)

var ApiFlags = []external.Flag{}

var ApiMeta = plugin.SubcommandMetadata{
	Description: "Scaffold a Kubernetes API with the declarative plugin",
	Examples:    "kubebuilder create api --plugins=go/v4,declarative/v1",
}

const (
	exampleManifestVersion = "0.0.1"
)

// ApiCmd handles all the logic for the `create api` subcommand
func ApiCmd(pr *external.PluginRequest) external.PluginResponse {
	pluginResponse := external.PluginResponse{
		Command:  "create api",
		Universe: pr.Universe,
	}

	// TODO(@em-r): add logic for handling `create api` command
	// GVK are required to scaffold templates, can be retrieved from PluginRequest.Args

	return pluginResponse
}

func apiScaffold() error {
	// Load the boilerplate
	boilerplate, err := os.ReadFile(filepath.Join("hack", "boilerplate.go.txt"))
	if err != nil {
		return fmt.Errorf("error updating scaffold: unable to load boilerplate: %w", err)
	}

	fs := machinery.Filesystem{
		FS: afero.NewOsFs(),
	}

	// Initialize the machinery.Scaffold that will write the files to disk
	scaffold := machinery.NewScaffold(fs,
		machinery.WithBoilerplate(string(boilerplate)),
	)

	//nolint:staticcheck
	err = scaffold.Execute(
		&templates.Types{},
		&templates.Controller{},
		&templates.Channel{ManifestVersion: exampleManifestVersion},
		&templates.Manifest{ManifestVersion: exampleManifestVersion},
	)

	if err != nil {
		return fmt.Errorf("error updating scaffold: %w", err)
	}

	// Update Dockerfile
	// nolint:staticcheck
	err = updateDockerfile()
	if err != nil {
		return err
	}

	return nil
}
