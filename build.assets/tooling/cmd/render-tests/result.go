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
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"

	"golang.org/x/exp/maps"
)

// separator for console output
const separator = "==================================================="

// action names used by the go test runner in its JSON output
const (
	actionPass   = "pass"
	actionFail   = "fail"
	actionSkip   = "skip"
	actionOutput = "output"
)

// covPattern matches output that contains package coverage values
var covPattern = regexp.MustCompile("\t" + `coverage: (\d+\.\d+)\% of statements`)

type counts struct {
	total int
	pass  int
	fail  int
	skip  int
}

func (c *counts) record(action string) {
	c.total++
	switch action {
	case actionPass:
		c.pass++
	case actionFail:
		c.fail++
	case actionSkip:
		c.skip++
	}
}

func (c counts) String() string {
	return fmt.Sprintf("%d passed, %d failed, %d skipped", c.pass, c.fail, c.skip)
}

// runResult records the results of an entire test run piped into render-tests.
type runResult struct {
	pkgCount  counts
	testCount counts
	packages  map[string]*packageResult
	reportBy  reportMode
}

// packageResult records the test results of a single Go package including the
// individual tests within that package.
type packageResult struct {
	name     string
	count    counts
	coverage *float64
	output   []string
	tests    map[string]*testResult
}

// testResult records the results of a single test.
type testResult struct {
	name   string
	count  counts
	output []string
}

func newRunResult(reportBy reportMode) *runResult {
	return &runResult{
		packages: map[string]*packageResult{},
		reportBy: reportBy,
	}
}

func newPackageResult(name string) *packageResult {
	return &packageResult{
		name:  name,
		tests: map[string]*testResult{},
	}
}

func newTestResult(name string) *testResult {
	return &testResult{
		name: name,
	}
}

func (rr *runResult) getPackage(name string) *packageResult {
	if pkg, ok := rr.packages[name]; ok {
		return pkg
	}
	pkg := newPackageResult(name)
	rr.packages[name] = pkg
	return pkg
}

func (rr *runResult) processTestEvent(te TestEvent) {
	pkg := rr.getPackage(te.Package)
	pkg.processTestEvent(te)

	if te.Test == "" {
		rr.pkgCount.record(te.Action)
	} else {
		rr.testCount.record(te.Action)
	}
}

func (rr *runResult) printTestResult(out io.Writer, te TestEvent) {
	if !isTestResult(te.Action) {
		return
	}

	// Report each completion of packages and tests when reporting by test
	if rr.reportBy == byTest {
		testname := te.Package
		if te.Test != "" {
			testname += "." + te.Test
		}
		fmt.Fprintf(out, "%s (in %6.2fs): %s\n", te.Action, te.ElapsedSeconds, testname)
	} else if rr.reportBy == byPackage && te.Test == "" {
		pkg := rr.getPackage(te.Package)
		covText := "------"
		if pkg.coverage != nil {
			covText = fmt.Sprintf("%5.1f%%", *pkg.coverage)
		}
		fmt.Fprintf(out, "%s %s (in %6.2fs): %s\n", te.Action, covText, te.ElapsedSeconds, pkg.name)
	}
}

func (rr *runResult) printSummary(out io.Writer) {
	fmt.Fprintln(out, separator)
	fmt.Fprintln(out, "Tests:", rr.testCount)
	fmt.Fprintln(out, "Packages:", rr.pkgCount)
	fmt.Fprintln(out, separator)

	if rr.testCount.fail == 0 {
		fmt.Fprintln(out, "All tests pass. Yay!")
		return
	}

	// Order the packages by name for consistent output ordering.
	pkgs := maps.Values(rr.packages)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].name < pkgs[j].name })

	printFailedTests(out, pkgs)
	fmt.Fprintln(out, separator)
	printFailedTestOutput(out, pkgs)
}

// printFailedTests prints a summary list of the failed tests and packages in
// the given packages.
func printFailedTests(out io.Writer, pkgs []*packageResult) {
	for _, pkg := range pkgs {
		if pkg.count.fail == 0 {
			continue
		}
		fmt.Fprintf(out, "FAIL: %s\n", pkg.name)
		for _, test := range pkg.tests {
			if test.count.fail == 0 {
				continue
			}
			fmt.Fprintf(out, "FAIL: %s\n", test.name)
		}
	}
}

// printFailedTestOutput prints the output of each failed package or test. Only
// print the package output if there is no test that failed (how can this
// happen?) so as to not swamp individual test output.
func printFailedTestOutput(out io.Writer, pkgs []*packageResult) {
	for _, pkg := range pkgs {
		testPrinted := false
		if pkg.count.fail == 0 {
			continue
		}
		for _, test := range pkg.tests {
			if test.count.fail == 0 {
				continue
			}
			printOutput(out, test.name, test.output)
			testPrinted = true
		}
		if !testPrinted {
			printOutput(out, pkg.name, pkg.output)
		}
	}
}

func printOutput(out io.Writer, test string, output []string) {
	fmt.Fprintf(out, "OUTPUT %s\n", test)
	fmt.Fprintln(out, separator)
	for _, line := range output {
		fmt.Fprint(out, line)
	}
	fmt.Fprintln(out, separator)
}

func (pr *packageResult) processTestEvent(te TestEvent) {
	if te.Action == actionOutput {
		// Record the output of package AND test against the package
		pr.output = append(pr.output, te.Output)
	}

	if te.Test != "" {
		tst := pr.getTest(pr.name + "." + te.Test)
		tst.processTestEvent(te)
		return
	}

	if te.Action == actionOutput {
		if matches := covPattern.FindStringSubmatch(te.Output); len(matches) > 0 {
			value, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				panic("Malformed coverage value: " + err.Error())
			}
			pr.coverage = &value
		}
	}

	if !isTestResult(te.Action) {
		return
	}

	pr.count.record(te.Action)

	// Delete test output of passed / skipped packages. Only save output of failures.
	if te.Action == actionPass || te.Action == actionSkip {
		pr.output = nil
	}
}

func (pr *packageResult) getTest(name string) *testResult {
	if tst, ok := pr.tests[name]; ok {
		return tst
	}
	tst := newTestResult(name)
	pr.tests[name] = tst
	return tst

}

func (tr *testResult) processTestEvent(te TestEvent) {
	if te.Action == actionOutput {
		tr.output = append(tr.output, te.Output)
	}

	if !isTestResult(te.Action) {
		return
	}

	tr.count.record(te.Action)

	// Delete test output of passed / skipped tests. Only save output of failures.
	if te.Action == actionPass || te.Action == actionSkip {
		tr.output = nil
	}
}

func isTestResult(action string) bool {
	return action == actionPass || action == actionFail || action == actionSkip
}
