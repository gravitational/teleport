package main

import (
	"fmt"
	"io"
)

// merge merges changelog1 and changelog2 by combining H2-level sections, which
// are specific to each released version.
func merge(changelog1, changelog2 io.Reader) ([]byte, error) {
	return nil, nil
}

func main() {
	fmt.Println("vim-go")
}
