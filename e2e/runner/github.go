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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v84/github"
	"golang.org/x/oauth2"
)

const commentMarker = "<!-- e2e-test-results -->"

func writeGitHubReport(e2eDir string) error {
	resultsPath := filepath.Join(e2eDir, "test-results", ".results.json")
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

	emitAnnotations(failures, flaky)

	if err := writeJobSummary(report, failures, flaky); err != nil {
		slog.Warn("could not write job summary", "error", err)
	}

	if err := managePRComment(report, failures, flaky); err != nil {
		slog.Warn("could not manage PR comment", "error", err)
	}

	return nil
}

func emitAnnotations(failures, flaky []pwFailure) {
	for _, f := range failures {
		msg := fmt.Sprintf("[%s] %s", f.projectName, firstErrorLine(f))
		fmt.Printf("::error file=%s,line=%d,col=%d::%s\n",
			escapeProp(f.file), f.line, f.column, escapeData(msg))
	}

	for _, f := range flaky {
		msg := fmt.Sprintf("[%s] Flaky: %s", f.projectName, f.title)
		fmt.Printf("::warning file=%s,line=%d,col=%d::%s\n",
			escapeProp(f.file), f.line, f.column, escapeData(msg))
	}
}

func writeJobSummary(report pwReport, failures, flaky []pwFailure) error {
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

func renderMarkdownReport(w io.Writer, report pwReport, failures, flaky []pwFailure) {
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
			fmt.Fprintf(w, "<details>\n<summary><code>-</code> <code>[%s]</code> <code>%s:%d</code> \u2014 %s</summary>\n<br>\n\n",
				escapeHTML(f.projectName), escapeHTML(f.file), f.line, escapeHTML(f.title))

			for _, r := range f.results {
				for _, e := range r.Errors {
					if e.Message != "" {
						writeCodeFence(w, e.Message)
					}
					if e.Snippet != "" {
						writeCodeFence(w, e.Snippet)
					}
				}
			}

			fmt.Fprint(w, "</details>\n\n")
		}
	}

	if len(flaky) > 0 {
		fmt.Fprint(w, "\n---\n\n#### Flaky\n\n")

		for _, f := range flaky {
			fmt.Fprintf(w, "<details>\n<summary><code>!</code> <code>[%s]</code> <code>%s:%d</code> \u2014 %s</summary>\n<br>\n\n",
				escapeHTML(f.projectName), escapeHTML(f.file), f.line, escapeHTML(f.title))

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

func managePRComment(report pwReport, failures, flaky []pwFailure) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	prNumber := prNumberFromEvent()
	if prNumber == 0 {
		slog.Info("not a pull request event, skipping PR comment")

		return nil
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		slog.Warn("GITHUB_REPOSITORY not set, skipping PR comment")

		return nil
	}

	owner, repoName, ok := strings.Cut(repo, "/")
	if !ok {
		return fmt.Errorf("invalid GITHUB_REPOSITORY format")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	existing := findExistingComment(ctx, client, owner, repoName, prNumber)

	hasIssues := len(failures) > 0 || len(flaky) > 0

	if !hasIssues {
		// If there are no issues, remove existing comment if present and return.
		if existing != nil {
			if _, err := client.Issues.DeleteComment(ctx, owner, repoName, existing.GetID()); err != nil {
				return fmt.Errorf("deleting existing PR comment: %w", err)
			}

			slog.Info("deleted existing E2E results PR comment")
		}

		return nil
	}

	var body strings.Builder

	body.WriteString(commentMarker + "\n")
	renderMarkdownReport(&body, report, failures, flaky)
	commentBody := body.String()

	if existing != nil {
		if _, _, err := client.Issues.EditComment(ctx, owner, repoName, existing.GetID(), &github.IssueComment{
			Body: github.Ptr(commentBody),
		}); err != nil {
			return fmt.Errorf("updating existing PR comment: %w", err)
		}

		slog.Info("updated E2E results PR comment")

		return nil
	}

	if _, _, err := client.Issues.CreateComment(ctx, owner, repoName, prNumber, &github.IssueComment{
		Body: github.Ptr(commentBody),
	}); err != nil {
		return fmt.Errorf("creating PR comment: %w", err)
	}

	slog.Info("created E2E results PR comment")

	return nil
}

func findExistingComment(ctx context.Context, client *github.Client, owner, repo string, prNumber int) *github.IssueComment {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			slog.Warn("could not list PR comments", "error", err)
			return nil
		}

		for _, c := range comments {
			if strings.Contains(c.GetBody(), commentMarker) {
				return c
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return nil
}

func prNumberFromEvent() int {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return 0
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		slog.Warn("could not read event payload", "path", eventPath, "error", err)
		return 0
	}

	var event struct {
		PullRequest *struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Warn("could not parse event payload", "error", err)
		return 0
	}

	if event.PullRequest == nil {
		return 0
	}

	return event.PullRequest.Number
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
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}

	fmt.Fprint(w, fence+"\n")
	fmt.Fprint(w, text)
	fmt.Fprint(w, "\n"+fence+"\n\n")
}

func firstErrorLine(f pwFailure) string {
	for _, r := range f.results {
		for _, e := range r.Errors {
			if e.Message != "" {
				if line, _, ok := strings.Cut(e.Message, "\n"); ok {
					return line
				}

				return e.Message
			}
		}
	}

	return f.title
}
