// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reference

import (
	"fmt"
	"os"
	"sort"
	"strings"

	template "github.com/DataDog/datadog-agent/pkg/template/text"

	metrics "github.com/gravitational/teleport/build.assets/tooling/cmd/metrics-ref-generator/metrics"
)

// GeneratorConfig is the user-facing configuration for the metric reference generator.
type GeneratorConfig struct {
	// SourcePath is the path to the root of the Go project directory.
	SourcePath string `yaml:"source"`
	// Destination is the file path where the generator writes the metric reference page.
	Destination string `yaml:"destination"`
	// Introduction is optional markdown rendered at the top of the page, before the automatically generated metrics reference.
	Introduction string `yaml:"introduction"`
	// Components maps metric name prefixes to component names.
	Components []ComponentConfig `yaml:"components"`
	// Sections describes how the metrics are organized into categorical sections. Metrics that do not
	// match any section are placed in an implicit "Other" section.
	Sections []SectionConfig `yaml:"sections"`
}

// ComponentConfig describes the configuration for mapping metric name prefixes to human-readable component names.
type ComponentConfig struct {
	Name    string   `yaml:"name"`
	Filters []string `yaml:"filters"`
}

// SectionConfig describes the configuration for a section of the generated metrics reference page.
type SectionConfig struct {
	// Title is the heading for this section.
	Title string `yaml:"title"`
	// Description is optional text rendered below the section heading.
	Description string `yaml:"description"`
	// Component overrides global component rules for metrics in this section.
	Component string `yaml:"component"`
	// Filters are prefixes matched against each metric's full name (namespace_subsystem_name).
	Filters []string `yaml:"filters"`
	// Metrics allows manually adding metrics that are not declared in the scanned Go source.
	Metrics []MetricConfig `yaml:"metrics"`
	// Sections enables adding nested sections.
	Sections []SectionConfig `yaml:"sections"`
}

// MetricConfig describes the configuration for a manually specified metric row.
type MetricConfig struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Component   string `yaml:"component"`
	Description string `yaml:"description"`
}

// pageContent is the top-level value passed to the reference template.
type pageContent struct {
	Introduction string
	Sections     []sectionData
}

// sectionData groups a set of related metrics under a heading.
type sectionData struct {
	SectionName string
	Description string
	Heading     string
	Fields      []metricRow
	Sections    []sectionData
}

// metricRow is a single row in the metrics table.
type metricRow struct {
	Name        string
	Type        string
	Component   string
	Description string
}

// Generate uses the provided configuration to write the metrics reference page to the destination
// path. prefix, e.g. "github.com/gravitational/teleport", is used to construct Go package paths
// while scanning the source tree.
func Generate(prefix string, conf GeneratorConfig, tmpl *template.Template) error {
	collectedMetrics, err := metrics.CollectMetrics(prefix, conf.SourcePath)
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	pc := buildPageContent(conf, collectedMetrics)
	pc.Introduction = conf.Introduction

	doc, err := os.Create(conf.Destination)
	if err != nil {
		return fmt.Errorf("cannot create output file at %v: %w", conf.Destination, err)
	}
	defer doc.Close()

	if err := tmpl.Execute(doc, pc); err != nil {
		return fmt.Errorf("cannot populate the metrics reference template: %w", err)
	}
	return nil
}

// buildPageContent organises metrics into sections as configured and returns
// the template data. Within each section metrics are sorted by full name.
func buildPageContent(conf GeneratorConfig, allMetrics []metrics.MetricInfo) pageContent {
	if len(conf.Sections) == 0 {
		return pageContent{Sections: []sectionData{buildSection(SectionConfig{}, "", allMetrics, conf.Components)}}
	}

	matched := make([]bool, len(allMetrics))
	sections := buildSections(conf.Sections, allMetrics, matched, conf.Components, 2)

	// Collect any metrics not matched by any section filter.
	var unmatched []metrics.MetricInfo
	for i, m := range allMetrics {
		if !matched[i] {
			unmatched = append(unmatched, m)
		}
	}
	if len(unmatched) > 0 {
		sections = append(sections, buildSection(SectionConfig{Title: "Other"}, "##", unmatched, conf.Components))
	}

	return pageContent{Sections: sections}
}

// buildSections recursively constructs sectionData for each section configuration,
// matching metrics against filters and organizing them hierarchically.
func buildSections(configs []SectionConfig, allMetrics []metrics.MetricInfo, matched []bool, components []ComponentConfig, level int) []sectionData {
	sections := make([]sectionData, 0, len(configs))
	for _, config := range configs {
		var subset []metrics.MetricInfo
		if len(config.Filters) > 0 {
			for i, metric := range allMetrics {
				if matchesAnyFilter(metric.FullName, config.Filters) {
					subset = append(subset, metric)
					matched[i] = true
				}
			}
		}

		headingLevel := min(level, 6)
		section := buildSection(config, strings.Repeat("#", headingLevel), subset, components)
		section.Sections = buildSections(config.Sections, allMetrics, matched, components, level+1)
		sections = append(sections, section)
	}
	return sections
}

// matchesAnyFilter checks if the given metric name matches any of the provided filters.
// Returns true if the metric name matches any of the filters, or if the filter list is empty.
func matchesAnyFilter(metricName string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.HasPrefix(metricName, filter) {
			return true
		}
	}
	return false
}

// configuredComponent returns the name of the first component whose filters match the given metric name.
// If no component matches, an empty string is returned.
func configuredComponent(metricName string, components []ComponentConfig) string {
	for _, component := range components {
		if matchesAnyFilter(metricName, component.Filters) {
			return component.Name
		}
	}
	return ""
}

// buildSection constructs a sectionData from a section configuration, a heading, and a slice of metrics.
// It determines the component for each metric, creates metric rows, and sorts them by full name.
func buildSection(config SectionConfig, heading string, metrics []metrics.MetricInfo, components []ComponentConfig) sectionData {
	if config.Title == "" {
		heading = ""
	}
	rows := make([]metricRow, 0, len(metrics)+len(config.Metrics))
	// Keep track of seen metrics to avoid duplicates.
	seen := make(map[string]struct{}, len(metrics)+len(config.Metrics))

	// Process each metric, determine its component, and create a metric row.
	for _, m := range metrics {
		component := config.Component
		if component == "" {
			component = configuredComponent(m.FullName, components)
		}
		if component == "" {
			component = m.Subsystem
		}
		if component == "" {
			component = m.Namespace
		}
		rows = append(rows, metricRow{
			Name:        fmt.Sprintf("`%s`", m.FullName),
			Type:        m.Type,
			Component:   component,
			Description: m.Help,
		})
		seen[m.FullName] = struct{}{}
	}
	// If there are manually defined metrics in the section configuration, process them as well.
	for _, metric := range config.Metrics {
		if _, ok := seen[metric.Name]; ok {
			continue
		}
		component := metric.Component
		if component == "" {
			component = config.Component
		}
		if component == "" {
			component = configuredComponent(metric.Name, components)
		}
		rows = append(rows, metricRow{
			Name:        fmt.Sprintf("`%s`", metric.Name),
			Type:        metric.Type,
			Component:   component,
			Description: metric.Description,
		})
	}
	// Sort the metric rows by their full name before returning the section data.
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return sectionData{
		SectionName: config.Title,
		Description: config.Description,
		Heading:     heading,
		Fields:      rows,
	}
}
