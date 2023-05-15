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

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"sigs.k8s.io/kubebuilder-declarative-pattern/plugin/v1/scaffolds"
	"sigs.k8s.io/kubebuilder/v3/pkg/plugin/external"
)

const (
	commandInit  = "init"
	commandFlags = "flags"
)

func returnError(err error) {
	response := external.PluginResponse{
		Error:     true,
		ErrorMsgs: []string{err.Error()},
	}

	output, err := json.Marshal(response)
	if err != nil {
		log.Fatalf("encountered error serializing output: %s | output: %s", err.Error(), output)
	}
	fmt.Printf("%s", output)
}

// Run will run the steps defined by the plugin
func Run() {
	// Kubebuilder makes requests to external plugin by writing STDIN.
	reader := bufio.NewReader(os.Stdin)

	input, err := io.ReadAll(reader)
	if err != nil {
		returnError(fmt.Errorf("encountered error reading from STDIN: %+v", err))
	}

	// Parsing request sent by kubebuilder into a PluginRequest instance.
	pluginRequest := &external.PluginRequest{}

	if err = json.Unmarshal(input, pluginRequest); err != nil {
		returnError(err)
	}

	// Run logic based on the command executed by Kubebuilder.
	var response external.PluginResponse
	switch pluginRequest.Command {
	case commandInit:
		response = scaffolds.InitCmd(pluginRequest)
	case commandFlags:
		response = FlagsCmd(pluginRequest)
	default:
		response = external.PluginResponse{
			Error:     true,
			ErrorMsgs: []string{"unknown subcommand:" + pluginRequest.Command},
		}
	}

	output, err := json.Marshal(response)
	if err != nil {
		returnError(err)
	}

	fmt.Printf("%s", output)
}
