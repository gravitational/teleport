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
	"slices"
	"strings"
)

const commentMarker = "<!-- e2e-test-results -->"

func writeGitHubReport(resultsPath string) error {
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("could not read test results: %w", err)
	}

	var report pwReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("could not parse test results: %w", err)
	}

	failures := collectTests(report.Suites, "unexpected")
	flaky := collectTests(report.Suites, "flaky")

	for i := range failures {
		failures[i].file = "e2e/tests/" + failures[i].file
	}
	for i := range flaky {
		flaky[i].file = "e2e/tests/" + flaky[i].file
	}

	mergedFailures := mergeFailures(failures)
	mergedFlaky := mergeFailures(flaky)

	emitAnnotations(mergedFailures, mergedFlaky)

	if err := writeJobSummary(report, mergedFailures, mergedFlaky); err != nil {
		slog.Warn("could not write job summary", "error", err)
	}

	if err := writePRCommentFile(resultsPath, report, mergedFailures, mergedFlaky); err != nil {
		slog.Warn("could not write PR comment file", "error", err)
	}

	return nil
}

type mergedFailure struct {
	title    string
	file     string
	line     int
	column   int
	projects []string
	results  []pwResult
}

type testKey struct {
	file     string
	line     int
	title    string
	firstErr string
}

func mergeFailures(failures []pwFailure) []mergedFailure {
	var order []testKey
	groups := map[testKey]*mergedFailure{}

	for _, f := range failures {
		browser, _, _ := strings.Cut(f.projectName, ":")
		k := testKey{file: f.file, line: f.line, title: f.title, firstErr: firstErrorMsg(f.results)}
		if m, ok := groups[k]; ok {
			if !slices.Contains(m.projects, browser) {
				m.projects = append(m.projects, browser)
			}
		} else {
			order = append(order, k)
			groups[k] = &mergedFailure{
				title:    f.title,
				file:     f.file,
				line:     f.line,
				column:   f.column,
				projects: []string{browser},
				results:  f.results,
			}
		}
	}

	merged := make([]mergedFailure, 0, len(order))
	for _, k := range order {
		merged = append(merged, *groups[k])
	}

	return merged
}

func firstErrorMsg(results []pwResult) string {
	for _, r := range results {
		for _, e := range r.Errors {
			if e.Message != "" {
				return stripANSI(e.Message)
			}
		}
	}

	return ""
}

func emitAnnotations(failures, flaky []mergedFailure) {
	for _, f := range failures {
		browsers := strings.Join(f.projects, ", ")
		msg := fmt.Sprintf("[%s] %s", browsers, firstErrorLine(f))
		fmt.Printf("::error file=%s,line=%d,col=%d::%s\n",
			escapeProp(f.file), f.line, f.column, escapeData(msg))
	}

	for _, f := range flaky {
		browsers := strings.Join(f.projects, ", ")
		msg := fmt.Sprintf("[%s] Flaky: %s", browsers, f.title)
		fmt.Printf("::warning file=%s,line=%d,col=%d::%s\n",
			escapeProp(f.file), f.line, f.column, escapeData(msg))
	}
}

func writeJobSummary(report pwReport, failures, flaky []mergedFailure) error {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		slog.Warn("GITHUB_STEP_SUMMARY not set, skipping job summary")

		return nil
	}

	var b strings.Builder

	renderMarkdownReport(&b, report, failures, flaky)

	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_STEP_SUMMARY: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(b.String()); err != nil {
		return fmt.Errorf("writing job summary: %w", err)
	}

	return nil
}

func renderMarkdownReport(w io.Writer, report pwReport, failures, flaky []mergedFailure) {
	fmt.Fprint(w, "### E2E Test Results\n\n")

	fmt.Fprint(w, "```diff\n")
	fmt.Fprintf(w, "+ %d passed\n", report.Stats.Expected)
	if report.Stats.Unexpected > 0 {
		fmt.Fprintf(w, "- %d failed\n", report.Stats.Unexpected)
	}
	if report.Stats.Flaky > 0 {
		fmt.Fprintf(w, "! %d flaky\n", report.Stats.Flaky)
	}
	if report.Stats.Skipped > 0 {
		fmt.Fprintf(w, "# %d skipped\n", report.Stats.Skipped)
	}
	fmt.Fprintf(w, "# %s\n", formatDuration(report.Stats.Duration))
	fmt.Fprint(w, "```\n")

	if len(failures) > 0 {
		fmt.Fprint(w, "\n#### Failures\n\n")

		for _, f := range failures {
			browsers := strings.Join(f.projects, ", ")
			fmt.Fprintf(w, "<details>\n<summary><code>[%s]</code> <code>%s:%d</code>\n\n**%s**</summary>\n\n\n",
				escapeHTML(browsers), escapeHTML(f.file), f.line, escapeHTML(f.title))

			writeFailureErrors(w, f.results)

			fmt.Fprint(w, "</details>\n\n")
		}
	}

	if len(flaky) > 0 {
		fmt.Fprint(w, "\n---\n\n#### Flaky\n\n")

		for _, f := range flaky {
			browsers := strings.Join(f.projects, ", ")
			fmt.Fprintf(w, "<details>\n<summary><code>[%s]</code> <code>%s:%d</code>\n\n**%s**</summary>\n\n\n",
				escapeHTML(browsers), escapeHTML(f.file), f.line, escapeHTML(f.title))

			for _, r := range f.results {
				if r.Retry == 0 {
					for _, e := range r.Errors {
						if e.Message != "" {
							fmt.Fprint(w, "**Initial failure:**\n")
							writeCodeFence(w, e.Message)
						}
					}
				}
			}

			fmt.Fprint(w, "</details>\n\n")
		}
	}

	if pr := ciPRNumber(); pr > 0 {
		fmt.Fprint(w, "---\n\n")
		fmt.Fprintf(w, "##### View full report\n```\n./e2e/run.sh --report %d\n```\n", pr)
	}
}

// writePRCommentFile writes the PR comment body to a file next to the results
// JSON so that a trusted workflow step can post it via `gh` without passing
// a write-scoped token to this (PR-built) binary.
func writePRCommentFile(resultsPath string, report pwReport, failures, flaky []mergedFailure) error {
	commentPath := filepath.Join(filepath.Dir(resultsPath), "pr-comment.md")

	hasIssues := len(failures) > 0 || len(flaky) > 0
	if !hasIssues {
		// Write an empty file to signal that the comment should be deleted.
		if err := os.WriteFile(commentPath, nil, 0o644); err != nil {
			return fmt.Errorf("writing empty PR comment file: %w", err)
		}

		slog.Info("wrote empty PR comment file (no issues)", "path", commentPath)

		return nil
	}

	var body strings.Builder
	body.WriteString(commentMarker + "\n")
	renderMarkdownReport(&body, report, failures, flaky)

	if err := os.WriteFile(commentPath, []byte(body.String()), 0o644); err != nil {
		return fmt.Errorf("writing PR comment file: %w", err)
	}

	slog.Info("wrote PR comment file", "path", commentPath)

	return nil
}

func escapeProp(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")

	return s
}

func escapeData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")

	return s
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	return s
}

func writeCodeFence(w io.Writer, text string) {
	text = stripANSI(text)
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}

	fmt.Fprint(w, fence+"\n")
	fmt.Fprint(w, text)
	fmt.Fprint(w, "\n"+fence+"\n\n")
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))

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

		b.WriteRune(r)
	}

	return b.String()
}

func writeFailureErrors(w io.Writer, results []pwResult) {
	if len(results) == 0 {
		return
	}

	// Show the first attempt's errors.
	first := results[0]
	for _, e := range first.Errors {
		if e.Message != "" {
			writeCodeFence(w, e.Message)
		}
		if e.Snippet != "" {
			writeCodeFence(w, e.Snippet)
		}
	}

	// Check retries.
	retries := 0
	var differentRetry *pwResult

	for i := 1; i < len(results); i++ {
		retries++
		if firstErrorMsg(results[i:i+1]) != firstErrorMsg(results[:1]) {
			differentRetry = &results[i]
		}
	}

	if retries == 0 {
		return
	}

	if differentRetry != nil {
		fmt.Fprintf(w, "**Retry #%d failed with a different error:**\n", differentRetry.Retry)
		for _, e := range differentRetry.Errors {
			if e.Message != "" {
				writeCodeFence(w, e.Message)
			}
			if e.Snippet != "" {
				writeCodeFence(w, e.Snippet)
			}
		}
	} else if retries == 1 {
		fmt.Fprint(w, "*Retried once and failed with the same error.*\n\n")
	} else {
		fmt.Fprintf(w, "*Retried %d times and failed with the same error.*\n\n", retries)
	}
}

func firstErrorLine(f mergedFailure) string {
	for _, r := range f.results {
		for _, e := range r.Errors {
			if e.Message != "" {
				msg := stripANSI(e.Message)
				if line, _, ok := strings.Cut(msg, "\n"); ok {
					return line
				}

				return msg
			}
		}
	}

	return f.title
}
