package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	docsbase := flag.String("docsbase", "", "path to the docs directory")
	workflows := flag.String("workflows", "", "path to the .github/workflows directory")
	flag.Parse()

	if *docsbase == "" || *workflows == "" {
		fmt.Fprintln(os.Stderr, "-docsbase and -workflows are required")
		flag.Usage()
		os.Exit(1)
	}

	if err := filepath.WalkDir(*workflows, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to check GitHub Actions workflows: %v\n", err)
		os.Exit(1)
	}
}
