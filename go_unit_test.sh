#!/usr/bin/env bash
set -e

echo "" > coverage.txt
for module in $(go list ./... | grep -v vendor); do
    go test -short -v -coverprofile=profile.out "$module"
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm -rf profile.out
    fi
done
