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
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/pass-pass-pass.in
	passPassPass string

	//go:embed testdata/pass-fail-pass.in
	passFailPass string

	//go:embed testdata/pass-fail-skip.in
	passFailSkip string

	//go:embed testdata/flaky-pass.in
	flakyPass string
	//go:embed testdata/flaky-fail-1.in
	flakyFail1 string
	//go:embed testdata/flaky-fail-4.in
	flakyFail4 string
	//go:embed testdata/flaky-fail-5.in
	flakyFail5 string
)

func TestHierarchy(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passFailSkip)

	pkgname := "example.com/package"
	require.Contains(t, rr.packages, pkgname)
	pkg := rr.packages[pkgname]
	require.Contains(t, pkg.tests, pkgname+".TestParse")
	require.Contains(t, pkg.tests, pkgname+".TestEmpty")
	require.Contains(t, pkg.tests, pkgname+".TestParseHostPort")
}

func TestStatus(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passFailSkip)

	require.Equal(t, 1, rr.testCount.pass)
	require.Equal(t, 1, rr.testCount.fail)
	require.Equal(t, 1, rr.testCount.skip)
	require.Equal(t, 1, rr.pkgCount.fail)
	pkgname := "example.com/package"
	pkg := rr.packages[pkgname]
	require.Equal(t, 1, pkg.count.fail)
	require.Equal(t, 1, pkg.tests[pkgname+".TestEmpty"].count.pass)
	require.Equal(t, 1, pkg.tests[pkgname+".TestParse"].count.fail)
	require.Equal(t, 1, pkg.tests[pkgname+".TestParseHostPort"].count.skip)
}

func TestSuccessOutput(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passPassPass)

	pkgname := "example.com/package"
	pkg := rr.packages[pkgname]
	require.Empty(t, pkg.output)
	require.Empty(t, pkg.tests[pkgname+".TestEmpty"].output)
	require.Empty(t, pkg.tests[pkgname+".TestParseHostPort"].output)
	require.Empty(t, pkg.tests[pkgname+".TestParse"].output)
}

func TestFailureOutput(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passFailSkip)

	pkgname := "example.com/package"
	pkg := rr.packages[pkgname]
	require.Empty(t, pkg.tests[pkgname+".TestEmpty"].output)
	require.Empty(t, pkg.tests[pkgname+".TestParseHostPort"].output)
	expectedTestOutput := []string{
		"=== RUN   TestParse\n",
		"=== PAUSE TestParse\n",
		"=== CONT  TestParse\n",
		"    addr_test.go:71: failed\n",
		"--- FAIL: TestParse (0.00s)\n",
	}
	expectedPkgOutput := []string{
		"FAIL\n",
		"\texample.com/package\tcoverage: 2.4% of statements\n",
		"FAIL\texample.com/package\t0.007s\n",
	}
	require.Equal(t, expectedTestOutput, pkg.tests[pkgname+".TestParse"].output)
	require.Equal(t, expectedPkgOutput, pkg.output)
}

func TestPrintTestResultByPackage(t *testing.T) {
	output := &bytes.Buffer{}
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byPackage, 0)
	for _, event := range events {
		rr.processTestEvent(event)
		rr.printTestResult(output, event)
	}

	expected := "fail   2.4% (in   0.01s): example.com/package\n"
	require.Equal(t, expected, output.String())
}

func TestPrintTestResultByTest(t *testing.T) {
	output := &bytes.Buffer{}
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byTest, 0)
	for _, event := range events {
		rr.processTestEvent(event)
		rr.printTestResult(output, event)
	}

	expected := `
skip (in   0.00s): example.com/package.TestParseHostPort
pass (in   0.00s): example.com/package.TestEmpty
fail (in   0.00s): example.com/package.TestParse
fail (in   0.01s): example.com/package
`[1:]
	require.Equal(t, expected, output.String())
}

func TestPrintSummaryNoFail(t *testing.T) {
	rr := newRunResult(byTest, 0)
	feedEvents(t, rr, passPassPass)

	output := &bytes.Buffer{}
	rr.printSummary(output)

	expected := `
===================================================
Tests: 3 passed, 0 failed, 0 skipped
Packages: 1 passed, 0 failed, 0 skipped
===================================================
All tests pass. Yay!
`[1:]
	require.Equal(t, expected, output.String())
}

func TestPrintSummaryFail(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passFailPass)

	output := &bytes.Buffer{}
	rr.printSummary(output)

	expected := `
===================================================
Tests: 1 passed, 1 failed, 1 skipped
Packages: 1 passed, 1 failed, 0 skipped
===================================================
FAIL: example.com/package
FAIL: example.com/package.TestParse
`[1:]
	require.Equal(t, expected, output.String())
}

func TestPrintFailedTestOutput(t *testing.T) {
	rr := newRunResult(byPackage, 0)
	feedEvents(t, rr, passFailPass)

	output := &bytes.Buffer{}
	rr.printFailedTestOutput(output)

	expected := `
OUTPUT example.com/package
===================================================
FAIL
	example.com/package	coverage: 2.4% of statements
FAIL	example.com/package	0.007s
===================================================
OUTPUT example.com/package.TestParse
===================================================
=== RUN   TestParse
=== PAUSE TestParse
=== CONT  TestParse
    addr_test.go:71: failed
--- FAIL: TestParse (0.00s)
===================================================
`[1:]
	require.Equal(t, expected, output.String())
}

func TestPrintFlakinessSummaryNoFail(t *testing.T) {
	rr := newRunResult(byFlakiness, 2) // top 2 failures only
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyPass)

	output := &bytes.Buffer{}
	rr.printFlakinessSummary(output)

	expected := "No flaky tests!\n"
	require.Equal(t, expected, output.String())
}

func TestPrintFlakinessSummaryFail(t *testing.T) {
	rr := newRunResult(byFlakiness, 3) // top 3 failures only (including packages)
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyFail1)
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyFail4)
	feedEvents(t, rr, flakyFail5)
	feedEvents(t, rr, flakyFail5)
	feedEvents(t, rr, flakyFail5)
	feedEvents(t, rr, flakyPass)
	feedEvents(t, rr, flakyFail1)
	feedEvents(t, rr, flakyPass)

	output := &bytes.Buffer{}
	rr.printFlakinessSummary(output)

	expected := `
FAIL(4/10): example.com/package3
FAIL(3/10): example.com/package3.Test5
FAIL(2/10): example.com/package1.Test1
`[1:]
	require.Equal(t, expected, output.String())
}

func strToEvents(t *testing.T, s string) []TestEvent {
	t.Helper()
	result := []TestEvent{}
	decoder := json.NewDecoder(strings.NewReader(s))
	for {
		event := TestEvent{}
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		result = append(result, event)
	}
	return result
}

func feedEvents(t *testing.T, rr *runResult, s string) {
	t.Helper()
	events := strToEvents(t, s)
	for _, event := range events {
		rr.processTestEvent(event)
	}
}
