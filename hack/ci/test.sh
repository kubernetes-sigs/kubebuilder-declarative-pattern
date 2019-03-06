#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/../..
cd "${REPO_ROOT}"

go get -u github.com/golang/dep/cmd/dep
dep ensure
go test sigs.k8s.io/kubebuilder-declarative-pattern/pkg/...
