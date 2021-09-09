#!/bin/bash

set -e

go mod tidy

# Make sure go test pass.
go test ./... -count=1

# Make sure Bazel test pass.
bazel test //... --nocache_test_results
