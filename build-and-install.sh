#!/bin/bash
set -e

echo "Building orbital..."
go build ./cmd/orbital

echo "Installing to GOPATH/bin..."
go install ./cmd/orbital

echo "Done."
