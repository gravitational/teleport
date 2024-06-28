package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
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

type parsedMD struct {
	Version     string
	Description string
}

var (
	releaseNotesTemplate = template.Must(template.New("release notes").Parse(tmpl))
)

type releaseNotesGenerator struct {
}

func (r *releaseNotesGenerator) generateReleaseNotes(md *os.File) (string, error) {
	p, err := r.parseMD(md)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := releaseNotesTemplate.Execute(&buff, p); err != nil {
		return "", trace.Wrap(err)
	}
	return buff.String(), nil
}

// parseMD is a simple implementation of a parser to extract the description from a changelog.
// Will scan for the first double header and pull the version from that.
// Will pull all information between the first and second double header for the description.
func (r *releaseNotesGenerator) parseMD(md *os.File) (parsedMD, error) {
	sc := bufio.NewScanner(md)
	var buff bytes.Buffer

	// Skip until first double header which should be version
	for sc.Scan() {
		if isDoubleHeader(sc.Text()) {
			break
		}
	}

	v := versionMatcher.FindString(sc.Text())
	if v == "" {
		return parsedMD{}, fmt.Errorf("a valid version was not found in first double header")
	}

	// Write everything until next header to buffer
	for sc.Scan() && !isDoubleHeader(sc.Text()) {
		if isEmpty(sc.Text()) { // skip empty lines
			continue
		}
		if _, err := buff.Write(append([]byte(sc.Text()), '\n')); err != nil {
			return parsedMD{}, err
		}
	}

	return parsedMD{
		Version:     v,
		Description: buff.String(),
	}, nil
}

func isDoubleHeader(line string) bool {
	return doubleHeaderMatcher.MatchString(line)
}

// isEmpty checks whether a line is just whitespace
func isEmpty(line string) bool {
	return strings.TrimSpace(line) == ""
}
