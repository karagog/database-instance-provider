#!/bin/bash

# Run this script before submitting any code to the repo or sending
# pull requests for review. It should be run from the source tree root.

set -e  # abort the script if any single command fails.

# Auto-format files.
find -name '*.go' | xargs gofmt -w

# Regenerate protobufs.
go generate

# Tidy up the go modules in case anything changed there.
go mod tidy

# Bring the BUILD.bazel files up to date with go module imports.
#
# NOTE: If you added new module imports via "go get", you may also have
# to update the repos in WORKSPACE.bazel like so:
#
#  $ bazel run //:gazelle -- update-repos --from_file=go.mod --prune
bazel run //:gazelle

# Make sure Bazel test passes. We use Bazel to test instead of "go test"
# because the BUILD files give us a richer configuration that specifies
# which tests we actually want to run.
#
# NOTE: This does not run tests that are tagged "manual" (e.g. the integration
# tests that require the provider service to be running). If you want to
# run the manual tests, you must specify them explicitly on the command line.
bazel test //... --build_manual_tests --nocache_test_results
