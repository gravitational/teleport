package main

import (
	nostructfieldassign "github.com/tigrato/teleport/build.assets/tools/linters/nostructfieldassign"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(nostructfieldassign.Analyzer)
}
