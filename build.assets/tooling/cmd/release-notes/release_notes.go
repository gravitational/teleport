package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

//go:embed template/release-notes.md.tmpl
var tmpl string

type tmplInfo struct {
	Version     string
	Description string
}

var (
	releaseNotesTemplate = template.Must(template.New("release notes").Parse(tmpl))
)

type releaseNotesGenerator struct {
	// releaseVersion is the version for the release.
	// This will be compared against the version present in the changelog.
	releaseVersion string
}

func (r *releaseNotesGenerator) generateReleaseNotes(md io.Reader) (string, error) {
	desc, err := r.parseMD(md)
	if err != nil {
		return "", err
	}

	info := tmplInfo{
		Version:     r.releaseVersion,
		Description: desc,
	}
	var buff bytes.Buffer
	if err := releaseNotesTemplate.Execute(&buff, info); err != nil {
		return "", trace.Wrap(err)
	}
	return buff.String(), nil
}

// parseMD is a simple implementation of a parser to extract the description from a changelog.
// Will scan for the first double header and pull the version from that.
// Will pull all information between the first and second double header for the description.
func (r *releaseNotesGenerator) parseMD(md io.Reader) (string, error) {
	sc := bufio.NewScanner(md)

	// Extract the first second-level heading
	var heading string
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "## ") {
			heading = strings.TrimSpace(strings.TrimPrefix(sc.Text(), "## "))
			break
		}
	}
	if err := sc.Err(); err != nil {
		return "", trace.Wrap(err)
	}
	if heading == "" {
		return "", trace.BadParameter("no second-level heading found in changelog")
	}

	// Expected heading would be something like "16.0.4 (MM/DD/YY)"
	parts := strings.SplitN(heading, " ", 2)
	if parts[0] != r.releaseVersion {
		return "", trace.BadParameter("changelog version number did not match expected version number: %q != %q", parts[0], r.releaseVersion)
	}

	// Write everything until next header to buffer
	var buff bytes.Buffer
	for sc.Scan() && !strings.HasPrefix(sc.Text(), "## ") {
		if _, err := fmt.Fprintln(&buff, sc.Text()); err != nil {
			return "", trace.Wrap(err)
		}
	}
	if err := sc.Err(); err != nil {
		return "", trace.Wrap(err)
	}

	return strings.TrimSpace(buff.String()), nil
}
