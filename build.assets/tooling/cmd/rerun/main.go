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

// Command rerun runs a command repeatedly until a timeout or number of runs completes.
//
//	Usage: rerun [-t timeout] [-n count] <command> [args...]
//
// Rerun runs <command> with [args...] <n> number of times or until <timeout>
// elapses since starting, whichever comes first. The command will be allowed
// to complete if <timeout> elapses, after which no more runs will be made.
//
// The exit code of command is the <command>'s exit code from the last run.
// If the <command> could not be run, then the exit code will be 1 and no
// more runs will be made.
//
// <timeout> is parsed as a [time.Duration string].
//
// If <count> is 0 (the default), then only the <timeout> will apply.
// If <timeout> is 0 (the default), then only the <count> will apply.
// If neither <count> or <timeout>, then the <command> will not be run.
//
// <command> is executed directly with [args...] as provided. If <command>
// does not contain any path separators, the search path is used to locate
// it. No shell is used to run <command>.
//
// [time.Duration string]: https://golang.org/pkg/time/#ParseDuration
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	count   = flag.Int("n", 0, "Maximum number of runs")
	timeout = flag.Duration("t", 0, "Rerun until timeout passes")
)

func usage() {
	fmt.Fprintln(flag.CommandLine.Output(), "usage: rerun [-t timeout] [-n count] <command> [args...]")
	flag.PrintDefaults()
}

type UsageError string

func (u UsageError) Error() string {
	return string(u)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	code, err := rerun(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var uerr UsageError
		if errors.As(err, &uerr) {
			usage()
		}
	}
	os.Exit(code)
}

func rerun(args []string) (int, error) {
	if len(args) < 1 {
		return 1, UsageError("no command supplied")
	}
	if *count < 0 {
		return 1, UsageError("run count must be a non-negative number")
	}
	if *timeout < 0 {
		return 1, UsageError("timeout must be a non-negative duration")
	}
	if *count == 0 && *timeout == 0 {
		return 0, nil
	}

	var (
		cmd      *exec.Cmd
		proc     *os.Process
		procLock sync.Mutex
		err      error
		done     = make(chan struct{})
		signals  = make(chan os.Signal, 1)
		now      = time.Now()
		endTime  = now.Add(*timeout)
	)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(done)
		sig := <-signals
		procLock.Lock()
		defer procLock.Unlock()
		if proc != nil {
			proc.Signal(sig)
		}
	}()
	for i := 0; !isDone(i, now, endTime, done); i++ {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

		// Immediately terminate if we cannot start the process. No point
		// re-running a command that cannot run.
		if err = cmd.Start(); err != nil {
			break
		}
		procLock.Lock()
		proc = cmd.Process
		procLock.Unlock()

		// Record the error for the case this is the last run. We return
		// that error.
		err = cmd.Wait()

		now = time.Now()
	}

	// We terminated due to some error trying to run the command. Return
	// with that error.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return 1, err
	}

	// Otherwise, just return the last exit code
	return cmd.ProcessState.ExitCode(), nil
}

// isDone returns true if we are done re-running the command; either we have
// reached the required number of runs, have reached the timeout, or the
// done channel is closed (via a signal).
func isDone(n int, now, endTime time.Time, ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return (*count > 0 && n > *count) || (*timeout > 0 && !now.Before(endTime))
	}
}
