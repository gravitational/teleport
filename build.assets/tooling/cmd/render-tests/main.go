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
				err = trace.Errorf("%s", line)
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
	}
	fmt.Fprintln(os.Stdout, separator)
	rr.printFailedTestOutput(os.Stdout)

	if rr.testCount.fail == 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
