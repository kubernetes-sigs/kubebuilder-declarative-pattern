#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/../..
cd "${REPO_ROOT}/hack"

go run smoketest.go
