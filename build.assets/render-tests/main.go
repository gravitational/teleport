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

// Package renderetests implements a filter that takes a stream of
// JSON fragmens as emitted by `go test -json` as input on stdin,
// then filters & renders them in arbitrarily complex ways
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"time"
)

type TestEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string
}

const (
	actionPass   = "pass"
	actionFail   = "fail"
	actionSkip   = "skip"
	actionOutput = "output"
)

func (e *TestEvent) FullName() string {
	if e.Test == "" {
		return e.Package
	}
	return e.Package + "." + e.Test
}

func readInput(input io.Reader, ch chan<- TestEvent) {
	decoder := json.NewDecoder(input)
	for {
		event := TestEvent{}

		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			close(ch)
			break
		}

		if err != nil {
			fmt.Printf("Error parsing JSON test record: %v\n", err)
			continue
		}

		ch <- event
	}
}

var separator = "==================================================="

func main() {

	testOutput := make(map[string][]string)
	failedTests := make(map[string][]string)
	actionCounts := make(map[string]int)

	events := make(chan TestEvent)
	go readInput(os.Stdin, events)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	keepGoing := true
	event := TestEvent{}

	for keepGoing {
		select {
		case <-signals:
			keepGoing = false

		case event, keepGoing = <-events:
			if !keepGoing {
				continue
			}

			testName := event.FullName()

			switch event.Action {
			case actionOutput:
				testOutput[testName] = append(testOutput[testName], event.Output)

			case actionPass, actionFail, actionSkip:
				// If this is a whole-package summary result
				if event.Test == "" {
					// only display package results as progress messages
					fmt.Printf("%s: %s\n", event.Action, event.Package)
				} else {
					// packages results don't count towards our test count.
					counts[event.Action] += 1

					// we want to preserve the log of our failed tests for
					// later examination
					if event.Action == actionFail {
						failedTests[testName] = testOutput[testName]
					}
				}
				// we don't need the test output any more, it's either in the
				// errors collection, or the test passed and we don't need to
				// display it.
				delete(testOutput, testName)
			}

		}
	}

	fmt.Println(separator)

	fmt.Printf("%d tests passed. %d failed, %d skipped\n",
		counts[actionPass], counts[actionFail], counts[actionSkip])

	fmt.Println(separator)

	if len(failedTests) == 0 {
		fmt.Println("All tests pass. Yay!")
		return
	}

	names := make([]string, 0, len(failedTests))
	for k := range failedTests {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, testName := range names {
		fmt.Printf("FAIL: %s\n", testName)
	}

	for _, testName := range names {
		fmt.Println(separator)
		fmt.Printf("TEST %s OUTPUT\n", testName)
		fmt.Println(separator)
		for _, l := range failedTests[testName] {
			fmt.Print(l)
		}
	}

	os.Exit(1)
}
