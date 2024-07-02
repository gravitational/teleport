/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
)

const (
	// timeNow is a convenience variable to signal that search should include PRs up to current time
	timeNow = ""
)

type parsedPR struct {
	// Summary is the changelog summary extracted from a PR
	Summary string
	Number  int
	URL     string
}

const (
	ossCLTemplate = `
{{- range . -}}
* {{.Summary}} [#{{.Number}}]({{.URL}})
{{ end -}}
`
	entCLTemplate = `
{{- range . -}}
* {{.Summary}}
{{ end -}}
`
)

var (
	// clPattern will match a changelog format with the summary as a subgroup.
	// e.g. will match a line "changelog: this is a changelog" with subgroup "this is a changelog".
	clPattern = regexp.MustCompile(`[Cc]hangelog: +(.*)`)

	ossCLParsedTmpl = template.Must(template.New("oss cl").Parse(ossCLTemplate))
	entCLParsedTmpl = template.Must(template.New("enterprise cl").Parse(entCLTemplate))
)

// pr is the expected output format of our search query
type pr struct {
	Body   string `json:"body,omitempty"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title,omitempty"`
	URL    string `json:"url,omitempty"`
}

type changelogGenerator struct {
	isEnt bool
	dir   string
}

// generateChangelog will pull a PRs from branch between two points in time and generate a changelog from them.
func (c *changelogGenerator) generateChangelog(branch, fromTime, toTime string) (string, error) {
	// searchQuery is based off of GitHub's search syntax
	searchQuery := fmt.Sprintf("base:%s merged:%s -label:no-changelog", branch, dateRangeFormat(fromTime, toTime))

	data, err := c.ghListPullRequests(searchQuery)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return c.toChangelog(data)
}

// ghListPullRequests is a wrapper around the `gh` command to list PRs
// searchQuery should follow the GitHub search syntax
func (c *changelogGenerator) ghListPullRequests(searchQuery string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("gh", "pr", "list", "--search", searchQuery, "--limit", "200", "--json", "number,url,title,body")
	cmd.Dir = c.dir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()

	if err != nil {
		return strings.TrimSpace(stderr.String()), trace.Wrap(err, "failed to get a list of prs")
	}

	return strings.TrimSpace(stdout.String()), nil
}

// toChangelog will take the output from the search and format it into a changelog.
func (c *changelogGenerator) toChangelog(data string) (string, error) {
	parsedList, err := parsePRList(data)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var tmpl *template.Template
	if c.isEnt {
		tmpl = entCLParsedTmpl
	} else {
		tmpl = ossCLParsedTmpl
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, parsedList); err != nil {
		return "", trace.Wrap(err)
	}

	return buff.String(), nil
}

// parsePRList parses raw output from gh cli
func parsePRList(data string) ([]parsedPR, error) {
	parsedList := []parsedPR{}

	// data should be in the format of a list of PR's
	var list []pr
	dec := json.NewDecoder(strings.NewReader(data))
	err := dec.Decode(&list)
	if err != nil {
		return parsedList, trace.Wrap(err)
	}

	for _, p := range list {
		found, clSummary := findChangelog(p.Body)
		if !found {
			// Pull out title and indicate no changelog found
			clSummary = fmt.Sprintf("NOCL: %s", p.Title)
		}
		parsed := parsedPR{
			Summary: prettierSummary(clSummary),
			Number:  p.Number,
			URL:     p.URL,
		}
		parsedList = append(parsedList, parsed)
	}
	return parsedList, nil
}

// findChangelog will parse a body of a PR to find a changelog.
func findChangelog(commentBody string) (found bool, summary string) {
	// If a match is found then we should get a non empty slice
	// 0 index will be the whole match including "changelog: *"
	// 1 index will be the subgroup match which does not include "changelog: "
	m := clPattern.FindStringSubmatch(commentBody)
	if len(m) > 1 {
		return true, m[1]
	}
	return false, ""
}

func prettierSummary(cl string) string {
	cl = strings.TrimSpace(cl)
	if !strings.HasSuffix(cl, ".") {
		cl += "."
	}
	return cl
}

// dateRangeFormat takes in a date range and will format it for GitHub search syntax.
// to can be empty and the format will be to search everything after from
func dateRangeFormat(from, to string) string {
	if to == "" {
		return fmt.Sprintf(">%s", from)
	}
	return fmt.Sprintf("%s..%s", from, to)
}
