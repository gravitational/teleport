/*
Copyright 2023 Gravitational, Inc.

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
)

func TestHierarchy(t *testing.T) {
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byPackage)
	for _, event := range events {
		rr.processTestEvent(event)
	}

	pkgname := "example.com/package"
	require.Contains(t, rr.packages, pkgname)
	pkg := rr.packages[pkgname]
	require.Contains(t, pkg.tests, pkgname+".TestParse")
	require.Contains(t, pkg.tests, pkgname+".TestEmpty")
	require.Contains(t, pkg.tests, pkgname+".TestParseHostPort")
}

func TestStatus(t *testing.T) {
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byPackage)
	for _, event := range events {
		rr.processTestEvent(event)
	}

	require.Equal(t, rr.passCount, 1)
	require.Equal(t, rr.failCount, 2) // +1 for package fail result
	require.Equal(t, rr.skipCount, 1)
	pkgname := "example.com/package"
	pkg := rr.packages[pkgname]
	require.Equal(t, pkg.status, "fail")
	require.Equal(t, pkg.tests[pkgname+".TestEmpty"].status, "pass")
	require.Equal(t, pkg.tests[pkgname+".TestParse"].status, "fail")
	require.Equal(t, pkg.tests[pkgname+".TestParseHostPort"].status, "skip")
}

func TestSuccessOutput(t *testing.T) {
	events := strToEvents(t, passPassPass)
	rr := newRunResult(byPackage)
	for _, event := range events {
		rr.processTestEvent(event)
	}

	pkgname := "example.com/package"
	pkg := rr.packages[pkgname]
	require.Empty(t, pkg.output)
	require.Empty(t, pkg.tests[pkgname+".TestEmpty"].output)
	require.Empty(t, pkg.tests[pkgname+".TestParseHostPort"].output)
	require.Empty(t, pkg.tests[pkgname+".TestParse"].output)
}

func TestFailureOutput(t *testing.T) {
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byPackage)
	for _, event := range events {
		rr.processTestEvent(event)
	}

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
		"=== RUN   TestParseHostPort\n",
		"=== PAUSE TestParseHostPort\n",
		"=== RUN   TestEmpty\n",
		"=== PAUSE TestEmpty\n",
		"=== RUN   TestParse\n",
		"=== PAUSE TestParse\n",
		"=== CONT  TestParseHostPort\n",
		"    addr_test.go:32: \n",
		"=== CONT  TestParse\n",
		"--- SKIP: TestParseHostPort (0.00s)\n",
		"=== CONT  TestEmpty\n",
		"    addr_test.go:71: failed\n",
		"--- PASS: TestEmpty (0.00s)\n",
		"--- FAIL: TestParse (0.00s)\n",
		"FAIL\n",
		"\texample.com/package\tcoverage: 2.4% of statements\n",
		"FAIL\texample.com/package\t0.007s\n",
	}
	require.Equal(t, pkg.tests[pkgname+".TestParse"].output, expectedTestOutput)
	require.Equal(t, pkg.output, expectedPkgOutput)
}

func TestPrintTestResultByPackage(t *testing.T) {
	output := &bytes.Buffer{}
	events := strToEvents(t, passFailSkip)
	rr := newRunResult(byPackage)
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
	rr := newRunResult(byTest)
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
	output := &bytes.Buffer{}
	events := strToEvents(t, passPassPass)
	rr := newRunResult(byTest)
	for _, event := range events {
		rr.processTestEvent(event)
	}
	rr.printSummary(output)

	expected := `
===================================================
4 tests passed, 0 failed, 0 skipped
===================================================
All tests pass. Yay!
`[1:]
	require.Equal(t, expected, output.String())
}

func TestPrintSummaryFail(t *testing.T) {
	output := &bytes.Buffer{}
	events := strToEvents(t, passFailPass)
	rr := newRunResult(byPackage)
	for _, event := range events {
		rr.processTestEvent(event)
	}
	rr.printSummary(output)

	expected := `
===================================================
2 tests passed, 2 failed, 1 skipped
===================================================
FAIL: example.com/package
FAIL: example.com/package.TestParse
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
