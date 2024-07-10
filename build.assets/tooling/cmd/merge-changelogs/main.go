package main

import (
	"fmt"
	"io"
)

// merge merges changelog1 and changelog2 by combining H2-level sections, which
// are specific to each released version. Returns the resulting merged document
// along with a list of messages indicating potential issues.
func merge(changelog1, changelog2 io.Reader) ([]byte, []string) {
	// TODO: Declare a map[string][][]byte, where the key is a version number and
	// the values are all H2s within the two changelogs with an H2 organized around
	// that version number

	// TODO: Read both changelogs into the map.

	// TODO: Initialize an array based on sorting the map keys

	// TODO: Iterate through the sorted array. For each map key, combine the H3s of
	// the values, using the later date, and add the combined values to the
	// final document.
	return nil, nil
}

func main() {
	fmt.Println("vim-go")
}
