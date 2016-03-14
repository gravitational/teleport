package main

import "github.com/gravitational/version"

func init() {
	// Reset base version to a custom one in case no tag has been created.
	// It will be reset with a git tag if there's one.
	version.Init("v0.0.1")
}

func main() {
	version.Print()
}
