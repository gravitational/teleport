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
	clPattern = regexp.MustCompile("[Cc]hangelog: +(.*)$")
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

// dateRangeFormat takes in a date range and will format it for Github search syntax
// to can be empty and the format will be to search everything after from
func dateRangeFormat(from, to string) string {
	if to == "" {
		return fmt.Sprintf(">%s", from)
	}
	return fmt.Sprintf("%s..%s", from, to)
}

// toChangelog will take the output from the search and format it into a changelog
func toChangelog(data string) (string, error) {
	var list []pr
	dec := json.NewDecoder(strings.NewReader(data))
	err := dec.Decode(&list)
	if err != nil {
		return "", trace.Wrap(err)
	}

	cl := ""
	for _, p := range list {
		line := ""
		if clPattern.Match([]byte(p.Body)) {
			line += clPattern.FindString(p.Body)
		} else {
			line += fmt.Sprintf("* NOCL: %s. ", p.Title)
		}
		line += fmt.Sprintf("[#%d](%s)", p.Number, p.Url)
		cl += line + "\n"
	}
	return cl, nil
}

// parseChangelogPRs will
func parseChangelogPRs(dir, branch, fromTime, toTime string) (string, error) {
	// searchQuery is based off of Github's search syntax
	searchQuery := fmt.Sprintf("base:%s merged:%s -label:no-changelog", branch, dateRangeFormat(fromTime, toTime))

	data, err := ghListPullRequests(dir, searchQuery)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return toChangelog(data)
}
