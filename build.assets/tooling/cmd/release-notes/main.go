package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin/v2"
)

var (
	version   = kingpin.Arg("version", "Version to be released").Required().String()
	changelog = kingpin.Arg("changelog", "Path to CHANGELOG.md").Required().String()
)

func main() {
	kingpin.Parse()

	clFile, err := os.Open(*changelog)
	if err != nil {
		log.Fatal(err)
	}

	gen := &releaseNotesGenerator{
		releaseVersion: *version,
	}

	notes, err := gen.generateReleaseNotes(clFile)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(notes)
}
