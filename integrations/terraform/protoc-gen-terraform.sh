#!/usr/bin/env sh

# This is a small wrapper that calls `go tool`. This is required because protoc doesn't support args directly.
# Using `go tool` allows us to version our tooling with the repo and ensure builds are reproducible.
# This also makes switching between branches easier because developers don't have to change the version installed locally.
# We can get rid of this when moving to `buf` as it supports plugin commands.

export GOWORK="off"
exec go tool protoc-gen-terraform