#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/../..
cd "${REPO_ROOT}"

go test sigs.k8s.io/kubebuilder-declarative-pattern/pkg/...
go test sigs.k8s.io/kubebuilder-declarative-pattern/examples/dashboard-operator/pkg/controller/...
