/*
Copyright 2021 Gravitational, Inc.

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

// Package main implements a filter that takes a stream of
// JSON fragmens as emitted by `go test -json` as input on stdin,
// then filters & renders them in arbitrarily complex ways
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

var covPattern = regexp.MustCompile(`^coverage: (\d+\.\d+)\% of statements`)

// matches `event` in src/cmd/internal/test2json/test2json.go
type TestEvent struct {
	Time           time.Time // encodes as an RFC3339-format string
	Action         string
	Package        string
	Test           string
	ElapsedSeconds float64 `json:"Elapsed"`
	Output         string
}

func (e *TestEvent) FullName() string {
	if e.Test == "" {
		return e.Package
	}
	return e.Package + "." + e.Test
}

// action names used by the go test runner in its JSON output
const (
	actionPass   = "pass"
	actionFail   = "fail"
	actionSkip   = "skip"
	actionOutput = "output"
)

// separator for console output
const separator = "==================================================="

func readInput(input io.Reader, ch chan<- TestEvent, errCh chan<- error) {
	defer close(ch)
	decoder := json.NewDecoder(input)
	for {
		event := TestEvent{}

		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			fmt.Printf("Error parsing JSON test record: %v\n", err)

			scanner := bufio.NewScanner(decoder.Buffered())
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					err = trace.Errorf(line)
					break
				}
			}

			errCh <- err

			return
		}

		ch <- event
	}
}

func main() {
	args := parseCommandLine()

	testOutput := newOutputMap()
	failedPackages := make(map[string]*packageOutput)
	coverage := make(map[string]float64)

	events := make(chan TestEvent)
	errors := make(chan error)
	go readInput(os.Stdin, events, errors)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

readloop:
	for {
		select {
		case <-signals:
			break readloop

		case err := <-errors:
			fmt.Printf("FATAL error: %q\n", err)

		case event, keepGoing := <-events:
			if !keepGoing {
				break readloop
			}

			testName := event.FullName()

			testOutput.record(event)

			if args.report == byTest {
				switch event.Action {
				case actionPass, actionFail, actionSkip:
					fmt.Printf("%s (in %6.2fs): %s\n", event.Action, event.ElapsedSeconds, event.FullName())
				}
			}

			// if this is whole-package summary result
			if event.Test == "" {
				switch event.Action {
				case actionOutput:
					if matches := covPattern.FindStringSubmatch(event.Output); len(matches) != 0 {
						value, err := strconv.ParseFloat(matches[1], 64)
						if err != nil {
							panic("Malformed coverage value: " + err.Error())
						}
						coverage[testName] = value
					}

				case actionFail:
					// cache the failed test output
					failedPackages[event.Package] = testOutput.getPkg(event.Package)
					fallthrough

				case actionPass, actionSkip:
					if args.report == byPackage {
						// extract and format coverage value
						covText := "------"
						if covValue, ok := coverage[testName]; ok {
							covText = fmt.Sprintf("%5.1f%%", covValue)
						}

						// only display package results as progress messages
						fmt.Printf("%s %s (in %6.2fs): %s\n", covText, event.Action, event.ElapsedSeconds, event.Package)
					}

					// Don't need this no more
					testOutput.deletePkg(event.Package)
				}
			}
		}
	}

	fmt.Println(separator)

	fmt.Printf("%d tests passed. %d failed, %d skipped\n",
		testOutput.actionCounts[actionPass], testOutput.actionCounts[actionFail], testOutput.actionCounts[actionSkip])

	fmt.Println(separator)

	if len(failedPackages) == 0 {
		fmt.Println("All tests pass. Yay!")
		os.Exit(0)
	}

	// Generate a sorted list of package names, so that we present the
	// packages that fail in a repeatable order.
	names := make([]string, 0, len(failedPackages))
	for k := range failedPackages {
		names = append(names, k)
	}
	sort.Strings(names)

	// Print a summary list of the failed tests and packages.
	for _, pkgName := range names {
		pkg := failedPackages[pkgName]

		fmt.Printf("FAIL: %s\n", pkgName)

		for testName := range pkg.failedSubtests {
			fmt.Printf("FAIL: %s.%s\n", pkgName, testName)
		}
	}

	fmt.Println(separator)

	// Print the output of each failed package or test. Note that we only print
	// the package output if there is no identifiable test that caused the
	// failure, as it will probably swamp the individual test output.
	for _, pkgName := range names {
		pkg := failedPackages[pkgName]

		if len(pkg.failedSubtests) == 0 {
			fmt.Printf("OUTPUT %s\n", pkgName)
			fmt.Println(separator)
			for _, l := range pkg.output {
				fmt.Print(l)
			}
			fmt.Println(separator)
		} else {
			for _, testName := range pkg.FailedTests() {
				fmt.Printf("OUTPUT %s.%s\n", pkgName, testName)
				fmt.Println(separator)
				for _, l := range pkg.failedSubtests[testName] {
					fmt.Print(l)
				}
				fmt.Println(separator)
			}
		}
	}

	os.Exit(1)
}
