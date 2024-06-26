package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var (
	// clPattern will match a changelog format with the summary as a subgroup.
	// e.g. will match a line "changelog: this is a changelog" with subgroup "this is a changelog".
	clPattern = regexp.MustCompile(`[Cc]hangelog: +(.*)`)
)

// pr is the expected output format of our search query
type pr struct {
	Body   string `json:"body,omitempty"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title,omitempty"`
	Url    string `json:"url,omitempty"`
}

// ghListPullRequests is a wrapper around the `gh` command to list PR's
// searchQuery should follow the Github search syntax
func ghListPullRequests(dir, searchQuery string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("gh", "pr", "list", "--search", searchQuery, "--limit", "200", "--json", "number,url,title,body")
	cmd.Dir = dir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()

	if err != nil {
		return strings.TrimSpace(stderr.String()), trace.Wrap(err, "failed to get a list of prs")
	}

	return strings.TrimSpace(stdout.String()), nil
}

// dateRangeFormat takes in a date range and will format it for Github search syntax.
// to can be empty and the format will be to search everything after from
func dateRangeFormat(from, to string) string {
	if to == "" {
		return fmt.Sprintf(">%s", from)
	}
	return fmt.Sprintf("%s..%s", from, to)
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

// toChangelog will take the output from the search and format it into a changelog.
func toChangelog(data string) (string, error) {
	// data should be in the format of a list of PR's
	var list []pr
	dec := json.NewDecoder(strings.NewReader(data))
	err := dec.Decode(&list)
	if err != nil {
		return "", trace.Wrap(err)
	}

	cl := ""
	for _, p := range list {
		found, clSummary := findChangelog(p.Body)
		if !found { // No summary found in body use title
			clSummary = fmt.Sprintf("NOCL: %s. ", p.Title)
		}
		clSummary = prettierSummary(clSummary)
		cl += fmt.Sprintf("* %s [#%d](%s)\n", clSummary, p.Number, p.Url)
	}
	return cl, nil
}

// generateChangelog will pull a PRs from branch between two points in time and generate a changelog from them.
func generateChangelog(dir, branch, fromTime, toTime string) (string, error) {
	// searchQuery is based off of Github's search syntax
	searchQuery := fmt.Sprintf("base:%s merged:%s -label:no-changelog", branch, dateRangeFormat(fromTime, toTime))

	data, err := ghListPullRequests(dir, searchQuery)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return toChangelog(data)
}
