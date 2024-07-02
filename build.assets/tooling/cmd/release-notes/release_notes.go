package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"html/template"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var (
	versionMatcher      = regexp.MustCompile(`\d+\.\d+\.\d+`)
	doubleHeaderMatcher = regexp.MustCompile(`^##\s+`)
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

func (r *releaseNotesGenerator) generateReleaseNotes(md *os.File) (string, error) {
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
func (r *releaseNotesGenerator) parseMD(md *os.File) (string, error) {
	sc := bufio.NewScanner(md)
	var buff bytes.Buffer

	// Skip until first double header which should be version
	for sc.Scan() {
		if isDoubleHeader(sc.Text()) {
			break
		}
	}

	if err := r.checkVersion(sc.Text()); err != nil {
		return "", trace.Wrap(err)
	}

	// Write everything until next header to buffer
	for sc.Scan() && !isDoubleHeader(sc.Text()) {
		if isEmpty(sc.Text()) { // skip empty lines
			continue
		}
		if _, err := buff.Write(append([]byte(sc.Text()), '\n')); err != nil {
			return "", trace.Wrap(err)
		}
	}
	return buff.String(), nil
}

// checkVersion will parse a version line from the changelog and ensure that it is correct.
func (r *releaseNotesGenerator) checkVersion(text string) error {
	v := versionMatcher.FindString(text)
	if v == "" {
		return trace.BadParameter("a correct version was not found in changelog")
	}

	if v != r.releaseVersion {
		return trace.BadParameter("version in changelog does not match configured version")
	}

	return nil
}

func isDoubleHeader(line string) bool {
	return doubleHeaderMatcher.MatchString(line)
}

// isEmpty checks whether a line is just whitespace
func isEmpty(line string) bool {
	return strings.TrimSpace(line) == ""
}
