package main

import (
	"fmt"
	"io"
)

// merge merges changelog1 and changelog2 by combining H2-level sections, which
// are specific to each released version. Returns the resulting merged document
// along with a list of messages indicating potential issues.
func merge(changelog1, changelog2 io.Reader) ([]byte, []string) {
	return nil, nil
}

func main() {
	fmt.Println("vim-go")
}
