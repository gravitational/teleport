package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
)

var (
	changelog = kingpin.Arg("changelog", "Path to CHANGELOG.md").Required().String()
)

func main() {
	kingpin.Parse()

	clFile, err := os.Open(*changelog)
	if err != nil {
		log.Fatal(err)
	}

	gen := &releaseNotesGenerator{}
	fmt.Println(gen.generateReleaseNotes(clFile))
}
