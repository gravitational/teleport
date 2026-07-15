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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

var (
	repoPath = kingpin.Flag("path", "Path to git repo").String()
	branch   = kingpin.Flag("branch", "Git base branch").Required().String()
	exclude  = kingpin.Flag("exclude", "Comma-separated list of exclude paths").Short('e').Strings()
	include  = kingpin.Flag("include", "Comma-separated list of include paths").Short('i').Strings()
	relative = kingpin.Flag("relative", "Returns paths relative to specified folder").String()
	skip     = kingpin.Flag("skip", "A space-delimited list of test names to skip").String()

	_ = kingpin.Command("diff", "Print diff in human-readable format")

	testCmd          = kingpin.Command("test", "Print go test flags to run changed tests")
	excludeUpdates   = testCmd.Flag("exclude-updates", "Exclude updated test methods").Short('u').Bool()
	onlyRunFlag      = testCmd.Flag("only-run-flag", "Show only -run flag").Short('r').Bool()
	escapeDollarSign = testCmd.Flag("escape-dollar-sign", "Output $ as $$").Short('d').Bool()
)

func main() {
	command := kingpin.Parse()

	testsToSkip := strings.Fields(*skip)

	// Set default git directory to cwd
	if repoPath == nil {
		p, err := os.Getwd()
		if err != nil {
			bail(trace.Errorf("Error getting current working directory: %v", err))
		}

		repoPath = &p
	}

	// Check if git is available
	err := gitIsAvailable()
	if err != nil {
		bail(trace.Wrap(err, "git is not available"))
	}

	start := time.Now()

	// Get fork commit ref
	ref, err := gitMergeBase(*repoPath, *branch)
	if err != nil {
		bail(trace.Wrap(err, "fork point might not exist"))
	}

	// Get git diff with fork commit
	changes, err := gitChanges(*repoPath, ref)
	if err != nil {
		bail(err)
	}

	// Get a list of changed files
	changedFiles, err := getChangedTestFilesFromDiff(changes, *exclude, *include)
	if err != nil {
		bail(err)
	}

	switch command {
	case "diff":
		diff(*repoPath, ref, changedFiles, time.Since(start))
	case "test":
		test(*repoPath, ref, changedFiles, testsToSkip)
	}
}

// diff prints diff for debug purposes
func diff(repoPath string, ref string, changedFiles []string, elapsed time.Duration) {
	fmt.Printf("Tests changed in %v compared to %v:\n\n", repoPath, ref)

	if len(changedFiles) == 0 {
		fmt.Println("No changes!")
		return
	}

	err := inspect(repoPath, ref, changedFiles, func(filename string, r CompareResult) {
		if !r.HasNew() && !r.HasChanged() {
			return
		}

		fmt.Printf("- %v:\n", filename)

		for _, n := range r.New {
			fmt.Printf("  +%v (%v)\n", n.Name, n.RefName)
		}

		for _, n := range r.Changed {
			fmt.Printf("  ~%v (%v)\n", n.Name, n.RefName)
		}

		fmt.Println()
	})
	if err != nil {
		bail(err)
	}

	fmt.Printf("Time elapsed: %s\n", elapsed)
}

// test builds and prints go test flags
func test(repoPath string, ref string, changedFiles []string, testsToSkip []string) {
	dirs := make(StringSet)
	methods := make([]string, 0)

	dollarSign := "$"
	if *escapeDollarSign {
		dollarSign = "$$"
	}

	err := inspect(repoPath, ref, changedFiles, func(filename string, r CompareResult) {
		if !r.HasNew() && !r.HasChanged() {
			return
		}

		dir, err := relDirName(filename, *relative)
		if err != nil {
			bail(err)
		}

		skipAll := slices.Contains(testsToSkip, "*")

		for _, n := range r.New {
			if (skipAll || slices.Contains(testsToSkip, n.RefName)) && !slices.Contains(testsToSkip, "!"+n.RefName) {
				log.Printf("-skipping %q (%s)\n", n.RefName, dir)
				continue
			}
			methods = append(methods, "^"+n.RefName+dollarSign)
			dirs[dir] = struct{}{}
		}

		if *excludeUpdates {
			return
		}

		for _, n := range r.Changed {
			if (skipAll || slices.Contains(testsToSkip, n.RefName)) && !slices.Contains(testsToSkip, "!"+n.RefName) {
				log.Printf("-skipping %q (%s)\n", n.RefName, dir)
				continue
			}
			methods = append(methods, "^"+n.RefName+dollarSign)
			dirs[dir] = struct{}{}
		}
	})
	if err != nil {
		bail(err)
	}

	if len(methods) == 0 || len(dirs) == 0 {
		return
	}

	fmt.Printf(`-run "%v"`, strings.Join(methods, "|"))
	if !*onlyRunFlag {
		fmt.Printf(" %v", strings.Join(dirs.Keys(), " "))
	}
}

// inspect iterates over changes in the repo
func inspect(repoPath string, ref string, changedFiles []string, fn func(string, CompareResult)) error {
	runners, err := findAllSuiteRunners(repoPath, changedFiles)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, filename := range changedFiles {
		file, err := gitGetFileFromRevision(repoPath, filename, ref)
		if err != nil {
			return trace.Errorf("%w : Error getting file %v from revision %v", err, filename, ref)
		}

		var forkPoint []Method

		if file != "" {
			forkPoint, err = parseMethodMap(filename, file, nil)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		head, err := parseMethodMap(filepath.Join(repoPath, filename), nil, runners)
		if err != nil {
			return trace.Wrap(err)
		}

		r := compare(forkPoint, head)

		fn(filename, r)
	}

	return nil
}

func relDirName(filename string, relative string) (string, error) {
	r, err := filepath.Rel(relative, filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return "./" + filepath.Dir(r), nil
}

// bail prints error and exits
func bail(err error) {
	fmt.Println(err)
	os.Exit(-1)
}
