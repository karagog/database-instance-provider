#!/bin/bash

# This script auto-generates all protobuf sources into the source languages we
# need. It is intended to be invoked by using "go generate" while at
# the root of the source tree.

# Uncomment this to see what commands are being run in the script.
# set -x

set -eu -o pipefail

# Exclude these paths from the proto file search.
EXCLUDE_PATHS=(
	'frontend/client'
	'frontend/templates'
)

echo "Removing existing generated files"

# Remove the generated Go files from the source tree.
rm -f $(find . -name '*.pb.go')

# Install protoc-gen-* plugins to do the compilation.
# Note that the versions are sticky to prevent spontaneous breakages.
# These versions should be kept in sync with the Bazel workspace to avoid
# version skew between build systems.
echo "go install google.golang.org/protobuf/cmd/protoc-gen-go"
(go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26)

echo "go install google.golang.org/protobuf/cmd/protoc-gen-go-grpc"
(go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1)

# List all the protobuf source files.
SOURCES=$(find . -name '*.proto')

# Run protoc individually on each protobuf file.
for src in ${SOURCES[@]}; do
	echo "protoc ${src}"
	protoc -I. \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		${src}
done
