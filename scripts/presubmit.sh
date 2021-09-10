#!/bin/bash

set -e  # abort the script if any single command fails.

# Tidy up the go modules in case anything changed there.
go mod tidy

# Bring the BUILD.bazel files up to date with go module imports.
#
# NOTE: If you added new module imports via "go get", you may also have
# to update the repos in WORKSPACE.bazel like so:
#
#  $ bazel run //:gazelle -- update-repos --from_file=go.mod
bazel run //:gazelle

# Make sure go test passes.
go test github.com/karagog/db-provider/... -count=1

# Make sure Bazel test passes. The result should be the same as "go test",
# unless the BUILD/WORKSPACE files are out of sync with the go packages.
# See the notes above about running Gazelle to fix this.
bazel test //... --nocache_test_results
