/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// printTestSummary reads the Playwright test results and prints them the same way that Playwright does, so
// we can show the overall test summary at the end after everything has exited and stopped logging.
func printTestSummary(e2eDir, resultsPath string) {
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		slog.Warn("could not read test results", "path", resultsPath, "error", err)
		return
	}

	var report pwReport
	if err := json.Unmarshal(data, &report); err != nil {
		slog.Warn("could not parse test results", "error", err)
		return
	}

	failed := collectTests(report.Suites, "unexpected")

	// rewrite the paths relative to the caller directory
	pathPrefix := ""
	if callerDir := os.Getenv("E2E_CALLER_DIR"); callerDir != "" && callerDir != e2eDir {
		if rel, err := filepath.Rel(callerDir, e2eDir); err == nil && rel != "." {
			pathPrefix = rel + "/"
		}
	}

	for i := range failed {
		failed[i].file = pathPrefix + "tests/" + failed[i].file
	}

	w := os.Stderr

	fmt.Fprintln(w)

	if len(failed) > 0 {
		for i, f := range failed {
			fmt.Fprintln(w, red(formatTestHeader(f, fmt.Sprintf("  %d) ", i+1))))

			printFailureErrors(w, f, e2eDir, pathPrefix)
		}

		fmt.Fprintln(w, red(fmt.Sprintf("  %d failed", len(failed))))

		for _, f := range failed {
			fmt.Fprintln(w, red(formatTestHeader(f, "    ")))
		}
	}

	if report.Stats.Flaky > 0 {
		fmt.Fprintln(w, yellow(fmt.Sprintf("  %d flaky", report.Stats.Flaky)))
	}

	if report.Stats.Skipped > 0 {
		fmt.Fprintln(w, yellow(fmt.Sprintf("  %d skipped", report.Stats.Skipped)))
	}

	if report.Stats.Expected > 0 {
		fmt.Fprintf(w, "  %s%s\n",
			green(fmt.Sprintf("%d passed", report.Stats.Expected)),
			dim(fmt.Sprintf(" (%s)", formatDuration(report.Stats.Duration))))
	}

	ciPR := ciPRNumber()

	var showReportCmd string
	if ciPR > 0 {
		showReportCmd = fmt.Sprintf("e2e/run.sh --report %d", ciPR)
	} else {
		showReportCmd = "pnpm show-report"
		if pathPrefix != "" {
			showReportCmd = fmt.Sprintf("(cd %s && pnpm show-report)", pathPrefix[:len(pathPrefix)-1])
		}
	}

	fmt.Fprintf(w, "\n  To open last HTML report run:\n\n    %s\n\n", cyan(showReportCmd))

	traces := collectTraces(failed, e2eDir)
	if len(traces) > 0 {
		fmt.Fprintln(w, "  Traces:")
		for _, t := range traces {
			var traceCmd string
			if ciPR > 0 {
				traceCmd = fmt.Sprintf("e2e/run.sh --test-results %d %s", ciPR, t.relPath)
			} else {
				traceCmd = fmt.Sprintf("pnpm exec playwright show-trace %s", pathPrefix+t.relPath)
			}
			fmt.Fprintf(w, "    %s\n      %s\n", dim(t.title), cyan(traceCmd))
		}
		fmt.Fprintln(w)
	}
}

func printFailureErrors(w io.Writer, f pwFailure, e2eDir, pathPrefix string) {
	for _, result := range f.results {
		if result.Retry > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, dim(separator(fmt.Sprintf("    Retry #%d", result.Retry))))
		}

		for _, e := range result.Errors {
			fmt.Fprintln(w)

			if e.Message != "" {
				for _, line := range strings.Split(e.Message, "\n") {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}

			if e.Snippet != "" {
				fmt.Fprintln(w)
				for _, line := range strings.Split(e.Snippet, "\n") {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}

			if e.Stack != "" {
				fmt.Fprintln(w)
				for _, line := range strings.Split(e.Stack, "\n") {
					fmt.Fprintf(w, "    %s\n", dim(line))
				}
			}
		}

		attachNum := 0
		var errorContextPath string

		for _, a := range result.Attachments {
			if strings.HasSuffix(a.Path, "error-context.md") {
				errorContextPath = a.Path
				continue
			}

			if a.Path == "" {
				continue
			}

			attachNum++

			fmt.Fprintln(w)
			fmt.Fprintln(w, dim(separator(fmt.Sprintf("    attachment #%d: %s (%s)", attachNum, a.Name, a.ContentType))))
			if rel, err := filepath.Rel(e2eDir, a.Path); err == nil {
				fmt.Fprintf(w, "    %s\n", dim(pathPrefix+rel))
			} else {
				fmt.Fprintf(w, "    %s\n", dim(a.Path))
			}
		}

		if attachNum > 0 {
			fmt.Fprintln(w, dim(separator("   ")))
		}

		if errorContextPath != "" {
			fmt.Fprintln(w)

			if rel, err := filepath.Rel(e2eDir, errorContextPath); err == nil {
				fmt.Fprintf(w, "    %s\n", dim("Error Context: "+pathPrefix+rel))
			} else {
				fmt.Fprintf(w, "    %s\n", dim("Error Context: "+errorContextPath))
			}
		}
	}

	fmt.Fprintln(w)
}

type pwReport struct {
	Suites []pwSuite `json:"suites"`
	Stats  pwStats   `json:"stats"`
}

type pwStats struct {
	Duration   float64 `json:"duration"`
	Expected   int     `json:"expected"`
	Unexpected int     `json:"unexpected"`
	Flaky      int     `json:"flaky"`
	Skipped    int     `json:"skipped"`
}

type pwSuite struct {
	Title  string    `json:"title"`
	File   string    `json:"file"`
	Suites []pwSuite `json:"suites"`
	Specs  []pwSpec  `json:"specs"`
}

type pwSpec struct {
	Title  string   `json:"title"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
	Tests  []pwTest `json:"tests"`
}

type pwTest struct {
	ProjectName string     `json:"projectName"`
	Status      string     `json:"status"`
	Results     []pwResult `json:"results"`
}

type pwResult struct {
	Status      string         `json:"status"`
	Retry       int            `json:"retry"`
	Errors      []pwError      `json:"errors"`
	Attachments []pwAttachment `json:"attachments"`
}

type pwAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}

type pwError struct {
	Message string `json:"message"`
	Snippet string `json:"snippet"`
	Stack   string `json:"stack"`
}

type pwFailure struct {
	title       string
	file        string
	line        int
	column      int
	projectName string
	results     []pwResult
}

type traceInfo struct {
	title   string
	relPath string
}

func collectTraces(failures []pwFailure, e2eDir string) []traceInfo {
	var traces []traceInfo
	for _, f := range failures {
		for _, result := range f.results {
			for _, a := range result.Attachments {
				if a.Name != "trace" || a.Path == "" {
					continue
				}
				rel, err := filepath.Rel(filepath.Join(e2eDir, "test-results"), a.Path)
				if err != nil {
					continue
				}
				title := fmt.Sprintf("[%s] %s › %s", f.projectName, f.file, f.title)
				if result.Retry > 0 {
					title += fmt.Sprintf(" (retry #%d)", result.Retry)
				}
				traces = append(traces, traceInfo{title: title, relPath: rel})
			}
		}
	}
	return traces
}

func collectTests(suites []pwSuite, status string) []pwFailure {
	var tests []pwFailure

	for _, suite := range suites {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				if test.Status == status {
					tests = append(tests, pwFailure{
						title:       spec.Title,
						file:        spec.File,
						line:        spec.Line,
						column:      spec.Column,
						projectName: test.ProjectName,
						results:     test.Results,
					})
				}
			}
		}

		tests = append(tests, collectTests(suite.Suites, status)...)
	}

	return tests
}

func formatTestHeader(f pwFailure, indent string) string {
	project := ""
	if f.projectName != "" {
		project = "[" + f.projectName + "] › "
	}

	location := fmt.Sprintf("%s:%d:%d", f.file, f.line, f.column)
	title := fmt.Sprintf("%s%s%s › %s", indent, project, location, f.title)

	return separator(title)
}

func formatDuration(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", int(ms))
	}

	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}

	minutes := int(seconds) / 60
	remaining := int(seconds) % 60

	return fmt.Sprintf("%dm %ds", minutes, remaining)
}

func separator(text string) string {
	if text != "" {
		text += " "
	}

	const columns = 100

	padding := columns - visibleLen(text)
	if padding < 0 {
		padding = 0
	}

	return text + dim(strings.Repeat("─", padding))
}

func visibleLen(s string) int {
	n := 0
	inEsc := false

	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}

		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}

		n++
	}

	return n
}

func red(s string) string {
	return "\033[31m" + s + "\033[39m"
}

func green(s string) string {
	return "\033[32m" + s + "\033[39m"
}

func yellow(s string) string {
	return "\033[33m" + s + "\033[39m"
}

func cyan(s string) string {
	return "\033[36m" + s + "\033[39m"
}

func dim(s string) string {
	return "\033[2m" + s + "\033[22m"
}
