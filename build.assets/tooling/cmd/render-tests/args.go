/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
