package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

type reportMode int

const (
	byPackage reportMode = 0
	byTest    reportMode = iota
)

const (
	byPackageName = "package"
	byTestName    = "test"
)

func (m *reportMode) String() string {
	switch *m {
	case byPackage:
		return byPackageName

	case byTest:
		return byTestName

	default:
		return fmt.Sprintf("Unknown filter mode %d", *m)
	}
}

func (m *reportMode) Set(text string) error {
	switch strings.TrimSpace(text) {
	case byPackageName:
		(*m) = byPackage

	case byTestName:
		(*m) = byTest

	default:
		return trace.Errorf("Invalid report mode %q", text)
	}

	return nil
}

type args struct {
	report reportMode
}

func parseCommandLine() args {
	reportMode := byPackage
	flag.Var(&reportMode, "report-by",
		fmt.Sprintf("test reporting mode [%s, %s]", byPackageName, byTestName))
	flag.Parse()

	return args{report: reportMode}
}
