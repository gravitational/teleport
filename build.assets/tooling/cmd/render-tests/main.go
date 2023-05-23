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
	"time"

	"github.com/gravitational/trace"
)

// matches `event` in src/cmd/internal/test2json/test2json.go
type TestEvent struct {
	Time           time.Time // encodes as an RFC3339-format string
	Action         string
	Package        string
	Test           string
	ElapsedSeconds float64 `json:"Elapsed"`
	Output         string
}

func readInput(input io.Reader, ch chan<- TestEvent, errCh chan<- error) {
	defer close(ch)
	decoder := json.NewDecoder(input)
	var err error
	for err == nil {
		event := TestEvent{}
		if err = decoder.Decode(&event); err == nil {
			ch <- event
		}
	}

	if !errors.Is(err, io.EOF) {
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
	}
}

func main() {
	args, err := parseCommandLine()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	events := make(chan TestEvent)
	errors := make(chan error)
	go readInput(os.Stdin, events, errors)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	var summaryOut io.Writer = os.Stdout
	if args.summaryFile != "" {
		f, err := os.Create(args.summaryFile)
		if err != nil {
			// Don't fatally exit, because we should still summarize the
			// results to stdout
			fmt.Fprintf(os.Stderr, "Could not create summary file: %v\n", err)
		} else {
			summaryOut = io.MultiWriter(os.Stdout, f)
			defer f.Close()
		}
	}

	rr := newRunResult(args.report, args.top)
	ok := true
	for ok {
		var event TestEvent
		select {
		case <-signals:
			ok = false

		case err := <-errors:
			fmt.Printf("FATAL error: %q\n", err)
			ok = false

		case event, ok = <-events:
			if ok {
				rr.processTestEvent(event)
				rr.printTestResult(os.Stdout, event)
			}
		}
	}

	if args.report == byFlakiness {
		rr.printFlakinessSummary(summaryOut)
	} else {
		rr.printSummary(summaryOut)
		fmt.Fprintln(os.Stdout, separator)
		rr.printFailedTestOutput(os.Stdout)
	}

	if rr.testCount.fail == 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
