/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

type reportMode int

const (
	byPackage reportMode = iota
	byTest
	byFlakiness
)

const (
	byPackageName   = "package"
	byTestName      = "test"
	byFlakinessName = "flakiness"
)

func (m *reportMode) String() string {
	switch *m {
	case byPackage:
		return byPackageName

	case byTest:
		return byTestName

	case byFlakiness:
		return byFlakinessName

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

	case byFlakinessName:
		(*m) = byFlakiness

	default:
		return trace.Errorf("Invalid report mode %q", text)
	}

	return nil
}

type args struct {
	report      reportMode
	top         int
	summaryFile string
}

func parseCommandLine() (args, error) {
	var a args
	flag.Var(&a.report, "report-by",
		fmt.Sprintf("test reporting mode [%s, %s, %s]", byPackageName, byTestName, byFlakinessName))
	flag.IntVar(&a.top, "top", 0, "top number of flaky tests to report [default 0 - all]")
	flag.StringVar(&a.summaryFile, "summary-file", "", "file to write summary to")
	flag.Parse()

	if a.top != 0 && a.report != byFlakiness {
		return args{}, trace.Errorf("-top <n> can only be used with -report-by flakiness")
	}

	return a, nil
}
