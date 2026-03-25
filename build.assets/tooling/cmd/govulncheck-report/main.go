// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// govulncheck-report wraps govulncheck, allows silences vulnerabilities and
// prints a brief report at the end.
//
// govulncheck must be on PATH.
//
// Usage: govulncheck-report -ignore=GO-2022-0635 -ignore=GO-2022-0646 -- [govulncheck flags here]
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

const programName = "govulncheck-report"

// Eg: "Vulnerability #1: GO-2025-4192"
var vulnRE = regexp.MustCompile(`^\s*Vulnerability #\d+: (GO-\d+-\d+)$`)

type govulnReport struct {
	govulnArgs   []string
	ignoredVulns []string
}

func newGovulnReport(thisArgs, govulnArgs []string) (*govulnReport, error) {
	cmd := &govulnReport{
		govulnArgs: govulnArgs,
	}

	fs := flag.NewFlagSet(programName, flag.ExitOnError)
	fs.Var(stringsFlag{dst: &cmd.ignoredVulns}, "ignore", "Vulnerability IDs to ignore")

	if err := fs.Parse(thisArgs); err != nil {
		// Unreachable, we have flag.ExitOnError set.
		panic(err)
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("arguments not supported, found trailing arguments: %v", fs.Args())
	}

	return cmd, nil
}

func (c *govulnReport) Run(ctx context.Context) (exitCode int, _ error) {
	// Make sure we can run properly.
	for _, arg := range c.govulnArgs {
		switch arg {
		case "-json", "--json", "-format", "--format":
			return 0, fmt.Errorf("%s doesn't support -json or non-text -format", programName)
		default:
			// OK, continue.
		}
	}

	pr, pw := io.Pipe()
	stdout := os.Stdout
	stderr := os.Stderr

	cmd := exec.CommandContext(ctx, "govulncheck", c.govulnArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = pw
	cmd.Stderr = stderr

	// Don't read until the goroutine below exits.
	var vulns []string

	// Parse and echo stdout.
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		scan := bufio.NewScanner(pr)
		for scan.Scan() {
			line := scan.Text()
			stdout.WriteString(line)
			stdout.WriteString("\n")

			if matches := vulnRE.FindStringSubmatch(line); len(matches) == 2 {
				vulns = append(vulns, matches[1])
			}
		}
		// No need to check scan.Err(). Once the pipe is drained we're done.
		_ = scan.Err()
	}()

	err := cmd.Run()
	pw.Close()
	pr.Close()
	<-doneC

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		exitCode = ee.ExitCode()
	} else {
		return 0, err // Unexpected error from cmd.Run()
	}
	// Do not print report if we found no vulnerabilities.
	if len(vulns) == 0 {
		return exitCode, nil
	}

	// Print report/redact exit code.
	slices.Sort(vulns)
	fmt.Fprintf(stderr, "\n%s:\n", programName)
	allIgnored := true
	for _, vuln := range vulns {
		var suffix string
		if ignored := slices.Contains(c.ignoredVulns, vuln); ignored {
			suffix = " (ignored)"
		} else {
			allIgnored = false
		}
		fmt.Fprintf(stderr, "  * %s%s\n", vuln, suffix)
	}
	if allIgnored {
		return 0, nil
	}

	return exitCode, nil
}

type stringsFlag struct {
	dst *[]string
}

func (f stringsFlag) Set(val string) error {
	*f.dst = append(*f.dst, val)
	return nil
}

func (f stringsFlag) String() string {
	if f.dst == nil {
		return ""
	}
	return strings.Join(*f.dst, ",")
}

func main() {
	thisArgs, govulnArgs := splitArgs(os.Args[1:]) // Remove program name.

	cmd, err := newGovulnReport(thisArgs, govulnArgs)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	exitCode, err := cmd.Run(ctx)
	if err != nil {
		panic(err)
	}
	os.Exit(exitCode)
}

func splitArgs(args []string) (thisArgs, govulnArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			govulnArgs = args[i+1:]
			break
		}
		thisArgs = append(thisArgs, arg)
	}
	return thisArgs, govulnArgs
}
