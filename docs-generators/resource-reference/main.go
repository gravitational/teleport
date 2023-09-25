package main

import (
	"flag"
	"fmt"
	"gen-resource-ref/reference"
	"os"
)

func main() {
	src := flag.String("source", ".", "the project directory in which to parse Go packages")
	out := flag.String("out", ".", "the path where the generator will write the resource reference")
	flag.Parse()

	outfile, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not prepare the output file for writing: %v", err)
		os.Exit(1)
	}

	err = reference.Generate(outfile, *src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not generate the resource reference: %v", err)
		os.Exit(1)
	}
}
