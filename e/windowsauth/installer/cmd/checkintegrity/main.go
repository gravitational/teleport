package main

import (
	"debug/pe"
	"fmt"
	"os"
)

// forceIntegrity is IMAGE_DLLCHARACTERISTICS_FORCE_INTEGRITY.
const forceIntegrity = 0x0080

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <dll>\n", os.Args[0])
		os.Exit(2)
	}
	path := os.Args[1]

	f, err := pe.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	h, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: %s is not a 64-bit image: %T\n", path, f.OptionalHeader)
		os.Exit(1)
	}
	if h.DllCharacteristics&forceIntegrity == 0 {
		fmt.Fprintf(os.Stderr,
			"error: %s DllCharacteristics = %#04x; want the %#04x bit set (see FORCE_INTEGRITY in the Makefile)\n",
			path, h.DllCharacteristics, forceIntegrity)
		os.Exit(1)
	}
}
