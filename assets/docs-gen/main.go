package main

import (
	"docs-gen/auditevents"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const auditEventsFileName = "audit-events.md"

func main() {

	in := flag.String("in", "", "Path to the root of the directory tree to use for analyzing Go files")
	out := flag.String("out", "", "Path to the directory to use for writing generated docs")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "You must provide values for the -from and -to flags.")
		flag.Usage()
		os.Exit(1)
	}

	gofiles := []*ast.File{}

	// Parse Go source files. We will use the results for all docs generators.
	if err := filepath.Walk(*in, func(pth string, i fs.FileInfo, _ error) error {
		if !strings.HasSuffix(i.Name(), ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), pth, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing Go source files: %v", err)
			os.Exit(1)
		}
		gofiles = append(gofiles, f)
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error walking gravitational/teleport: %v", err)
		os.Exit(1)
	}

	f, err := os.OpenFile(path.Join(*out, auditEventsFileName), os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to file %v: %v\n", auditEventsFileName, err)
		os.Exit(1)
	}
	if err := auditevents.GenerateAuditEventsTable(f, gofiles); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating docs file %v: %v\n", auditEventsFileName, err)
		os.Exit(1)
	}

}
