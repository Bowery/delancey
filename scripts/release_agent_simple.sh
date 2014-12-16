#!/bin/bash

# Get the full path to the parent of this script.
source="${BASH_SOURCE[0]}"
while [[ -h "${source}" ]]; do source="$(readlink "${source}")"; done
root="$(cd -P "$(dirname "${source}")/.." && pwd)"
CGO_ENABLED=0

# Get the version we're building.
version="$(cat VERSION)"
echo "Version: ${version}"

mkdir -p "${root}/build"

# Compile AWS Upload Utility
cd "${root}/scripts"
go build -o "${root}/build/util" util.go
cd -

# Compile Delancey
go get -d
GOOS=linux GOARCH=amd64 go build -o "${root}/build/bowery-agent"

# Upload to http://bowery.sh/bowery-agent
"${root}/build/util" aws bowery.sh "${root}/build"
