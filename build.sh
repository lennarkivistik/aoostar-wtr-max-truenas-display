#!/bin/bash

# Usage: ./build.sh <GOOS> <GOARCH> <VERSION>
# Example: ./build.sh linux amd64 v1.0.0

set -e

# Check for correct number of arguments
if [ $# -ne 3 ]; then
    echo "Usage: $0 <GOOS> <GOARCH> <VERSION>"
    echo "Example: $0 linux amd64 v1.0.0"
    exit 1
fi

GOOS=$1
GOARCH=$2
VERSION=$3

# Determine if the target is Windows for executable extension
OUTPUT_NAME="wtr-display-${GOOS}-${GOARCH}-${VERSION}"
if [ "$GOOS" = "windows" ]; then
    OUTPUT_NAME+=".exe"
fi

# Define the output directory
BUILD_DIR="build/${GOOS}-${GOARCH}"
mkdir -p "$BUILD_DIR"

# Set environment variables for cross-compilation
export GOOS=$GOOS
export GOARCH=$GOARCH
export CGO_ENABLED=0

# For MIPS64 variants, you might need to set GOMIPS. Adjust as needed.
if [[ "$GOARCH" == "mips64le" || "$GOARCH" == "mips64" ]]; then
    export GOMIPS=softfloat
fi

# Build the binary with version information
go build -o "$BUILD_DIR/$OUTPUT_NAME" -ldflags "-X main.version=$VERSION" .

echo "Built $OUTPUT_NAME for $GOOS/$GOARCH at $BUILD_DIR/$OUTPUT_NAME"

