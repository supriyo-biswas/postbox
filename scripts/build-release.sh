#!/usr/bin/env bash

set -euo pipefail

build_darwin_arm64() {
    if [[ $(uname -sm) != "Darwin arm64" ]]; then
        echo "This step can only be run on MacOS arm64, skipping..."
    else
        echo "Building darwin/arm64"
        go build -o "data/releases/$1/postbox-darwin-arm64"
    fi
}

build_linux_x86_64() {
    echo "Building linux/x86_64"
    docker run --platform linux/amd64 --rm -v "$PWD:/src" -w /src golang:1.21.3 \
        go build -o "data/releases/$1/postbox-linux-x86_64"
}

build_linux_arm64() {
    echo "Building linux/aarch64"
    docker run --platform linux/arm64 --rm -v "$PWD:/src" -w /src golang:1.21.3 \
        go build -o "data/releases/$1/postbox-linux-aarch64"
}

tag=$(git describe --exact-match --tags "$(git log -n1 --pretty='%h')")
mkdir -p "data/releases/$tag"

build_darwin_arm64 "$tag"
build_linux_arm64 "$tag"
build_linux_x86_64 "$tag"

git push origin "$tag"
gh release create "$tag" --title "$tag" --notes "" "data/releases/$tag"/*
