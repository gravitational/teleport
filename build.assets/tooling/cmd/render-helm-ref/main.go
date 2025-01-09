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
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// tagRegex matches tagged values with kind. For example:
// # full.path.valueName(kind) -- controls how to do X
var tagRegex = regexp.MustCompile(`^\s*#\s*(.*)\((.+)\)\s+--\s*(.*)$`)

// tagRegex matches tagged values without kind. For example:
// # full.path.valueName -- controls how to do X
var tagRegexNoKind = regexp.MustCompile(`^\s*#\s*(.*)\s+--\s*(.*)$`)

// defaultRegex matches the default override tag. For example:
// # @default -- see values.yaml
var defaultRegex = regexp.MustCompile(`^\s*# @default -- (.*)$`)

func main() {
	var chartPath string
	var outputPath string
	flag.StringVar(&chartPath, "chart", "", "Path of the chart.")
	flag.StringVar(&outputPath, "output", "-", "Path of the generated markdown reference, '-' means stdout.")
	flag.Parse()

	ctx := context.Background()
	if chartPath == "" {
		slog.ErrorContext(ctx, "chart path must be specified")
		os.Exit(1)
	}

	reference, err := parseAndRender(chartPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed parsing chart and rendering reference", "error", err)
		os.Exit(1)
	}

	if outputPath == "-" {
		fmt.Print(string(reference))
		os.Exit(0)
	}
	err = os.WriteFile(outputPath, reference, 0o644)
	if err != nil {
		slog.ErrorContext(ctx, "failed writing file", "error", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "File successfully written", "file_path", outputPath)
}

func parseAndRender(chartPath string) ([]byte, error) {
	chartValues := chartPath + "/" + "values.yaml"

	// First, we check if we can load all the data we need.
	// We need both the Helm chart (for automatic default detection)
	chrt, err := loader.Load(chartPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load chart")
	}

	// And the raw values.yaml (for the documentation in comments)
	valuesYAML, err := os.ReadFile(chartValues)
	if err != nil {
		return nil, trace.Wrap(err, "failed to open values '%s'", chartValues)
	}

	// We parse the YAML and extract all the documented values form its comments
	var n yaml.Node
	err = yaml.Unmarshal(valuesYAML, &n)
	if err != nil {
		return nil, trace.Wrap(err, "failed to unmarshall values")
	}
	values := processYAMLNode(&n)

	// Then, we backfill the default value when possible
	for _, value := range values {
		// We can skip the default detection by setting no kind. This is useful when
		// we are also documenting subfields and don't want an ugly Type/Default table.
		if value.Kind != "" && value.Default == "" {
			defaultValue, err := getDefaultForValue(value.Name, chrt.Values)
			if err != nil {
				slog.WarnContext(context.Background(), "failed to look up default value",
					"value", value.Name,
					"error", err,
				)
			} else {
				value.Default = string(defaultValue)
			}
		}
	}

	// Finally we render
	reference, err := renderTemplate(values)
	return reference, trace.Wrap(err, "failed to render template")
}

func processYAMLNode(node *yaml.Node) []*Value {
	// The YAML structure does not represent the value hierarchy
	// So we process all comments the same way and don't care about their position
	var values []*Value
	if value := grabValue(node.HeadComment); value != nil {
		values = append(values, value)
	}
	if value := grabValue(node.LineComment); value != nil {
		values = append(values, value)
	}
	if value := grabValue(node.FootComment); value != nil {
		values = append(values, value)
	}
	if len(node.Content) != 0 {
		for _, subNode := range node.Content {
			values = append(values, processYAMLNode(subNode)...)
		}
	}
	return values
}

type Value struct {
	Name string
	Kind string

	Description string
	Default     string
}

type state struct {
	isDescription bool
	description   strings.Builder

	value *Value
}

// grabValue walks through a comment and checks if it has a value tag.
// Once the value tag is found, everything after it will be part of the description.
func grabValue(comment string) *Value {
	if comment == "" {
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(comment))

	var line string

	s := state{}
	for scanner.Scan() {
		line = scanner.Text()
		if !s.isDescription {
			// We are not yet in a comment containing documentation
			match, name, kind, remain := matchTag(line)
			if !match {
				// no tag on this line, we skip it
				continue
			}
			// start of a value documentation
			s.isDescription = true
			s.value = &Value{Name: name, Kind: kind}
			s.description.WriteString(remain)
			s.description.WriteRune('\n')
			continue
		}
		// We already saw a tag on a previous line
		// If we find a default override tag we process it, else we just add the
		// line to the existing value description.
		if match, defaultValue := matchDefaultTag(line); match {
			s.value.Default = defaultValue
			continue
		}
		s.description.WriteString(cleanLine(line))
		s.description.WriteRune('\n')
	}

	if s.isDescription {
		s.value.Description = strings.TrimSpace(s.description.String())
	}
	return s.value
}

func matchTag(line string) (match bool, name, kind, remain string) {
	// If kind is specified
	subMatches := tagRegex.FindStringSubmatch(line)
	if len(subMatches) == 4 && subMatches[1] != "" {
		return true, subMatches[1], subMatches[2], subMatches[3]
	}
	// If kind is not specified
	subMatches = tagRegexNoKind.FindStringSubmatch(line)
	if len(subMatches) == 3 && subMatches[1] != "" {
		return true, subMatches[1], "", subMatches[2]
	}
	return false, "", "", ""
}

func matchDefaultTag(line string) (bool, string) {
	subMatches := defaultRegex.FindStringSubmatch(line)
	if len(subMatches) != 2 {
		return false, ""
	}
	return true, subMatches[1]
}

func cleanLine(line string) string {
	line2 := strings.TrimSpace(line)
	if len(line2) < 3 {
		return ""
	}
	if line2[0] != '#' {
		slog.WarnContext(context.Background(), "Misformatted line", "line", line)
		return ""
	}
	return line2[2:]
}

// getDefaultForValue takes a value detected from the comments, and looks up its
// default value in the Helm chart.
func getDefaultForValue(valueName string, chartValues map[string]interface{}) ([]byte, error) {
	parts := strings.Split(valueName, ".")
	// Check if this is a nested value
	if len(parts) > 1 {
		chartValue, ok := chartValues[parts[0]]
		if !ok {
			// Stop if the value is unknown
			return nil, trace.NotFound("value '%s' not found", parts[0])
		}

		// The value name is part0.part1...partX
		// We expect the detected Helm value to be a map, else we don't know
		// how to access "part1...partX"
		if subValue, ok := chartValue.(map[string]interface{}); ok {
			return getDefaultForValue(strings.Join(parts[1:], "."), subValue)
		}
		return nil, trace.CompareFailed("value %s cannot be cast to a map", parts[0])

	}
	// If the value is known we marshall it as JSON
	if chartValue, ok := chartValues[parts[0]]; ok {
		return json.Marshal(chartValue)
	}
	return nil, trace.NotFound("value '%s' not found", parts[0])
}
