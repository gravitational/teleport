// Package main implements the tmig binary — the Teleport scope migration tool.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
