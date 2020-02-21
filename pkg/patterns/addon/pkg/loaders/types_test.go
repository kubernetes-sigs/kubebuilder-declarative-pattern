/*
Copyright 2019 The Kubernetes Authors.

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

package loaders

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"sigs.k8s.io/kustomize/api/filesys"
)

func TestFSRepository_LoadManifest(t *testing.T) {

	fSys := filesys.MakeFsOnDisk()
	baseDir := "/tmp/packages/nginx/1.2.3/"
	err := fSys.MkdirAll(baseDir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	defer fSys.RemoveAll(baseDir)

	filePath := filepath.Join(baseDir, "manifest.yaml")
	var manifestStr = `
	kind: Deployment
	metadata:
	  labels:
		app: nginx2
	  name: foo
	  annotations:
		app: nginx2
	spec:
	  replicas: 1
	---
	kind: Service
	metadata:
	  name: foo
	  annotations:
		app: nginx
	spec:
	  selector:
		app: nginx`

	err = fSys.WriteFile(filePath, []byte(manifestStr))
	if err != nil {
		t.Fatalf("writing manifest file: %v", err)
	}

	expected := map[string]string{
		filePath: manifestStr,
	}

	ctx := context.Background()
	var fs = NewFSRepository("/tmp")

	actual, err := fs.LoadManifest(ctx, "nginx", "1.2.3")

	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %+v but got %+v", expected, actual)
	}
}
