#!/bin/bash
set -e

echo "Building orbit-cli..."
go build ./cmd/orbit-cli

echo "Installing to GOPATH/bin..."
go install ./cmd/orbit-cli

echo "Done."
