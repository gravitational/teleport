package main

// nolint
import (
	"crypto/boring"
	"os"
)

func main() {
	if !boring.Enabled() {
		os.Exit(1)
	}
}
