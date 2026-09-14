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
	"os"
	"path/filepath"
	"testing"

	template "github.com/DataDog/datadog-agent/pkg/template/text"
	metrics "github.com/gravitational/teleport/build.assets/tooling/cmd/metrics-ref-generator/metrics"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	moduleRoot := t.TempDir()
	// CollectMetrics requires a Go module root to resolve package paths.
	writeTestFile(t, moduleRoot, "go.mod", "module example.com/project\n")
	writeTestFile(t, moduleRoot, "lib/metrics.go", `
package metrics

import (
	prometheus "github.com/prometheus/client_golang/prometheus"
)

var requests = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "test",
	Name: "requests",
	Help: "Number of requests.",
})
`)

	newTemplate := func(t *testing.T, text string) *template.Template {
		t.Helper()
		tmpl, err := template.New("reference").Parse(text)
		require.NoError(t, err)
		return tmpl
	}

	validConfig := GeneratorConfig{
		SourcePath:   filepath.Join(moduleRoot, "lib"),
		Destination:  filepath.Join(moduleRoot, "output.mdx"),
		Introduction: "Metrics:\n",
		Sections:     []SectionConfig{{Title: "Requests", Filters: []string{"test_"}}},
	}

	t.Run("successfully generates the metrics reference", func(t *testing.T) {
		err := Generate("example.com/project", validConfig, newTemplate(t, "{{ .Introduction }}{{ range .Sections }}{{ .SectionName }}:{{ range .Fields }}{{ .Name }}{{ end }}{{ end }}"), "config.yaml")
		require.NoError(t, err)

		output, err := os.ReadFile(validConfig.Destination)
		require.NoError(t, err)
		require.Equal(t, "Metrics:\nRequests:`test_requests`", string(output))
	})

	t.Run("returns Go source loading errors", func(t *testing.T) {
		conf := validConfig
		conf.SourcePath = filepath.Join(moduleRoot, "missing")
		require.ErrorContains(t, Generate("example.com/project", conf, newTemplate(t, ""), "config.yaml"), "loading Go source files")
	})

	t.Run("returns page content build errors", func(t *testing.T) {
		conf := validConfig
		conf.Sections = nil
		require.ErrorContains(t, Generate("example.com/project", conf, newTemplate(t, ""), "config.yaml"), "failed to build page content")
	})

	t.Run("returns output file creation errors", func(t *testing.T) {
		conf := validConfig
		conf.Destination = filepath.Join(moduleRoot, "missing", "output.mdx")
		require.ErrorContains(t, Generate("example.com/project", conf, newTemplate(t, ""), "config.yaml"), "cannot create output file at")
	})

	t.Run("returns template population errors", func(t *testing.T) {
		require.ErrorContains(t, Generate("example.com/project", validConfig, newTemplate(t, "{{ template \"missing\" . }}"), "config.yaml"), "cannot populate the metrics reference template")
	})
}

func TestBuildPageContent(t *testing.T) {
	cases := []struct {
		description    string
		config         GeneratorConfig
		metrics        []metrics.MetricInfo
		configPath     string
		expected       pageContent
		errorSubstring string
	}{
		{
			description: "builds sections for matched metrics",
			config: GeneratorConfig{
				Components: []ComponentConfig{{Name: "Auth", Filters: []string{"test_auth_"}}},
				Sections: []SectionConfig{{
					Title:       "Requests",
					Description: "Authentication request metrics.",
					Filters:     []string{"test_"},
				}},
			},
			metrics: []metrics.MetricInfo{{
				Namespace: "test",
				Subsystem: "auth",
				FullName:  "test_auth_requests",
				Type:      "counter",
				Help:      "Requests | total.",
			}},
			expected: pageContent{Sections: []sectionData{{
				SectionName: "Requests",
				Description: "Authentication request metrics.",
				Heading:     "##",
				Fields: []metricRow{{
					Name:        "`test_auth_requests`",
					Type:        "counter",
					Component:   "Auth",
					Description: "Requests \\| total.",
				}},
				Sections: []sectionData{},
			}}},
		},
		{
			description:    "returns an error when sections are missing",
			configPath:     "metrics.yaml",
			errorSubstring: "No sections defined in the configuration file (metrics.yaml)",
		},
		{
			description: "returns an error when metrics are not covered by filters",
			config: GeneratorConfig{Sections: []SectionConfig{{
				Title:   "Requests",
				Filters: []string{"other_"},
			}},
			},
			metrics:        []metrics.MetricInfo{{FullName: "test_requests"}},
			configPath:     "metrics.yaml",
			errorSubstring: "The following metrics are not covered by section filters: \ntest_requests.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual, err := buildPageContent(tc.config, tc.metrics, tc.configPath)
			if tc.errorSubstring != "" {
				require.ErrorContains(t, err, tc.errorSubstring)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestBuildSections(t *testing.T) {
	cases := []struct {
		description     string
		configs         []SectionConfig
		allMetrics      []metrics.MetricInfo
		matched         []bool
		components      []ComponentConfig
		level           int
		expected        []sectionData
		expectedMatched []bool
	}{
		{
			description: "groups filtered metrics into nested sections and records matches",
			configs: []SectionConfig{{
				Title:   "Authentication",
				Filters: []string{"auth_"},
				Sections: []SectionConfig{{
					Title:     "Database",
					Component: "Database",
					Filters:   []string{"db_"},
				}},
			}},
			allMetrics: []metrics.MetricInfo{
				{FullName: "auth_requests", Type: "counter", Help: "Requests."},
				{FullName: "db_connections", Type: "gauge", Help: "Connections."},
				{FullName: "cache_hits", Type: "counter", Help: "Hits."},
			},
			matched:    make([]bool, 3),
			components: []ComponentConfig{{Name: "Auth", Filters: []string{"auth_"}}},
			level:      2,
			expected: []sectionData{{
				SectionName: "Authentication",
				Heading:     "##",
				Fields: []metricRow{{
					Name:        "`auth_requests`",
					Type:        "counter",
					Component:   "Auth",
					Description: "Requests.",
				}},
				Sections: []sectionData{{
					SectionName: "Database",
					Heading:     "###",
					Fields: []metricRow{{
						Name:        "`db_connections`",
						Type:        "gauge",
						Component:   "Database",
						Description: "Connections.",
					}},
					Sections: []sectionData{},
				}},
			}},
			expectedMatched: []bool{true, true, false},
		},
		{
			description: "caps headings at level six",
			configs: []SectionConfig{{
				Title:   "Deep section",
				Filters: []string{"metric_"},
			}},
			allMetrics: []metrics.MetricInfo{{FullName: "metric_value", Type: "gauge"}},
			matched:    make([]bool, 1),
			level:      7,
			expected: []sectionData{{
				SectionName: "Deep section",
				Heading:     "######",
				Fields: []metricRow{{
					Name: "`metric_value`",
					Type: "gauge",
				}},
				Sections: []sectionData{},
			}},
			expectedMatched: []bool{true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual := buildSections(tc.configs, tc.allMetrics, tc.matched, tc.components, tc.level)
			require.Equal(t, tc.expected, actual)
			require.Equal(t, tc.expectedMatched, tc.matched)
		})
	}
}

func TestMatchesAnyFilter(t *testing.T) {
	cases := []struct {
		description string
		filters     []string
		metric      string
		expected    bool
	}{
		{
			description: "matches when metric starts with filter",
			filters:     []string{"auth_"},
			metric:      "auth_requests",
			expected:    true,
		},
		{
			description: "does not match when metric does not start with filter",
			filters:     []string{"auth_"},
			metric:      "db_connections",
			expected:    false,
		},
		{
			description: "returns true when filters list is empty",
			filters:     []string{},
			metric:      "any_metric",
			expected:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual := matchesAnyFilter(tc.metric, tc.filters)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestConfiguredComponent(t *testing.T) {
	cases := []struct {
		description string
		metricName  string
		components  []ComponentConfig
		expected    string
	}{
		{
			description: "returns the correct component for a given metric",
			metricName:  "auth_requests",
			components: []ComponentConfig{
				{Name: "Auth", Filters: []string{"auth_"}},
				{Name: "Database", Filters: []string{"db_"}},
			},
			expected: "Auth",
		},
		{
			description: "returns empty string when no component matches",
			metricName:  "cache_hits",
			components: []ComponentConfig{
				{Name: "Auth", Filters: []string{"auth_"}},
				{Name: "Database", Filters: []string{"db_"}},
			},
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual := configuredComponent(tc.metricName, tc.components)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestEscapeMetricDescription(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "escapes '|' characters",
			input:       `this | that`,
			expected:    `this \| that`,
		},
		{
			description: "does not modify string without '|' characters",
			input:       `this has no pipe`,
			expected:    `this has no pipe`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual := escapeMetricDescription(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestBuildSection(t *testing.T) {
	cases := []struct {
		description string
		config      SectionConfig
		heading     string
		metrics     []metrics.MetricInfo
		components  []ComponentConfig
		expected    sectionData
	}{
		{
			description: "builds sorted rows using configured components and skips duplicate manual metrics",
			config: SectionConfig{
				Title:       "Requests",
				Description: "Request metrics.",
				Component:   "Section component",
				Metrics: []MetricConfig{
					{Name: "auth_requests", Type: "gauge", Description: "Ignored duplicate."},
					{Name: "manual_metric", Type: "gauge", Description: "Manual | metric."},
				},
			},
			heading: "##",
			metrics: []metrics.MetricInfo{
				{FullName: "auth_requests", Type: "counter", Subsystem: "auth", Help: "Authentication requests."},
				{FullName: "db_requests", Type: "counter", Subsystem: "db", Help: "Database requests."},
			},
			components: []ComponentConfig{{Name: "Auth", Filters: []string{"auth_"}}},
			expected: sectionData{
				SectionName: "Requests",
				Description: "Request metrics.",
				Heading:     "##",
				Fields: []metricRow{
					{Name: "`auth_requests`", Type: "counter", Component: "Section component", Description: "Authentication requests."},
					{Name: "`db_requests`", Type: "counter", Component: "Section component", Description: "Database requests."},
					{Name: "`manual_metric`", Type: "gauge", Component: "Section component", Description: "Manual \\| metric."},
				},
			},
		},
		{
			description: "omits heading for untitled sections and falls back to namespace",
			config:      SectionConfig{},
			heading:     "##",
			metrics: []metrics.MetricInfo{{
				FullName:  "requests",
				Namespace: "teleport",
				Type:      "counter",
			}},
			expected: sectionData{
				Fields: []metricRow{{
					Name:      "`requests`",
					Type:      "counter",
					Component: "teleport",
				}},
			},
		},
		{
			description: "uses a configured component for manual metrics without an explicit component",
			config: SectionConfig{
				Title: "Manual metrics",
				Metrics: []MetricConfig{{
					Name: "auth_sessions",
					Type: "gauge",
				}},
			},
			heading:    "##",
			components: []ComponentConfig{{Name: "Auth", Filters: []string{"auth_"}}},
			expected: sectionData{
				SectionName: "Manual metrics",
				Heading:     "##",
				Fields: []metricRow{{
					Name:      "`auth_sessions`",
					Type:      "gauge",
					Component: "Auth",
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual := buildSection(tc.config, tc.heading, tc.metrics, tc.components)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func writeTestFile(t *testing.T, root, relativePath, contents string) string {
	t.Helper()
	filePath := filepath.Join(root, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte(contents), 0o600))
	return filePath
}
