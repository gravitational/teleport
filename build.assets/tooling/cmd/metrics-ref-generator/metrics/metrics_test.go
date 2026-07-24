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

package metrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectMetrics(t *testing.T) {
	cases := []struct {
		description string
		// files contains the file paths and their corresponding contents for the test case.
		files map[string]string
		// scanRoot specifies the root directory to start scanning for metrics.
		scanRoot string
		// expectedMetrics contains the list of metrics expected to be collected for the test case.
		expectedMetrics []MetricInfo
		// errorSubstring specifies a substring that should be present in the error message if an error is expected.
		errorSubstring string
	}{

		{
			description: "type declarations and Prometheus metrics",
			files: map[string]string{
				"go.mod": "module example.com/project\n\ngo 1.24\n",
				"constants.go": `
				package project

                const (
                	MetricNamespace = "teleport"
                	MetricVersion = 2
                )
                `,
				"metrics/metrics.go": `
				package metrics
                
                import (
                	"fmt"
                
                	project "example.com/project"
                	other "example.com/other"
                	prom "github.com/prometheus/client_golang/prometheus"
					teleportmetrics "example.com/project/lib/observability/metrics"
                )
                
                const localSubsystem = "auth"
				const metricsSubsystem = "cache"
                
                type MetricLabels struct {
                	Cluster string
                }
                
                type (
                	FirstType struct{}
                	SecondType struct{}
                )
                
                var (
                	// requestCount should not replace explicit help.
                	requestCount = prom.NewCounter(prom.CounterOpts{
                		Namespace: project.MetricNamespace,
                		Subsystem: localSubsystem,
                		Name: fmt.Sprintf("requests_v%d", project.MetricVersion),
                		Help: "Number of requests.",
                	})
                	// activeSessions measures the number of active sessions
                	// across all clusters.
                	activeSessions = prom.NewGaugeVec(prom.GaugeOpts{
                		Name: "active_sessions",
                	}, []string{"cluster"})
                	userLogins = prom.NewCounterVec(prom.CounterOpts{
                		Namespace: project.MetricNamespace,
                		Name:      "user_login_per_client",
                		Help: "The number of successful user authentications by client tool, " +
                			"and proxy that handled the request.",
                	}, []string{"client"})
                	latency = prom.NewHistogram(prom.HistogramOpts{Name: "latency_seconds"})
                	duration = prom.NewSummary(prom.SummaryOpts{Name: "duration_seconds"})
                	notPrometheus = other.NewGauge(other.GaugeOpts{Name: "ignored"})
                	noOptions = prom.NewCounter()
                	nonLiteralOptions = prom.NewCounter(options)
                )
                
                func newClientMetrics() {
                	_ = &struct {
                		dialErrors any
                		connections any
                	}{
                		dialErrors: prom.NewCounterVec(prom.CounterOpts{
                			Namespace: "proxy_peer",
                			Subsystem: "client",
                			Name:      "dial_error_total",
                			Help:      "Total number of errors encountered dialing peer proxies.",
                		}, []string{"error_type"}),
                		connections: prom.NewGaugeVec(prom.GaugeOpts{
                			Namespace: "proxy_peer",
                			Subsystem: "client",
                			Name:      "connections",
                			Help:      "Number of currently opened connections to proxy peer servers.",
                		}, []string{"state"}),
                	}
                }

				func registerMetrics(registry *teleportmetrics.Registry) {
					metricName := "operations"
					_ = prom.NewCounter(prom.CounterOpts{
						Namespace: registry.Namespace(),
						Subsystem: registry.Subsystem(),
						Name:      fmt.Sprintf("service_%s", metricName),
						Help:      fmt.Sprintf("Operations for %s", runtimeService()),
					})
				}
                `,
				"metrics/ignored_test.go": `
				package metrics

                import prom "github.com/prometheus/client_golang/prometheus"
                
                var ignoredTestMetric = prom.NewCounter(prom.CounterOpts{Name: "ignored_test"})
                `,
				"metrics/default_import.go": `
				package metrics

                import "github.com/prometheus/client_golang/prometheus"
                
                var defaultImport = prometheus.NewCounter(prometheus.CounterOpts{Name: "default_import"})
                `,
				"metrics/standalone_comment.go": `
				package metrics
                 
                import "github.com/prometheus/client_golang/prometheus"
                
                // standaloneMetric tracks standalone operations.
                var standaloneMetric = prometheus.NewCounter(prometheus.CounterOpts{Name: "standalone_metric"})
                `,
				"metrics/lookalike_import.go": `
				package metrics
                
                import prometheus "example.com/prometheus"
                
                var lookalikeImport = prometheus.NewCounter(prometheus.CounterOpts{Name: "lookalike_import"})
                `,
				"metrics/missing_import.go": `
				package metrics
                
                var missingImport = prometheus.NewCounter(prometheus.CounterOpts{Name: "missing_import"})
                `,
				"outside.go": `
				package project
                
                import prom "github.com/prometheus/client_golang/prometheus"
                
                var outsideScanRoot = prom.NewCounter(prom.CounterOpts{Name: "outside"})
                `,
			},
			scanRoot: "metrics",
			expectedMetrics: []MetricInfo{
				{
					FilePath:  "metrics.go",
					Namespace: "teleport",
					Subsystem: "auth",
					Name:      "requests_v2",
					Help:      "Number of requests.",
					FullName:  "teleport_auth_requests_v2",
					Type:      "counter",
				},
				{
					FilePath: "metrics.go",
					Name:     "active_sessions",
					Help:     "Measures the number of active sessions across all clusters.",
					FullName: "active_sessions",
					Type:     "gauge",
				},
				{
					FilePath:  "metrics.go",
					Namespace: "teleport",
					Name:      "user_login_per_client",
					Help:      "The number of successful user authentications by client tool, and proxy that handled the request.",
					FullName:  "teleport_user_login_per_client",
					Type:      "counter",
				},
				{
					FilePath: "metrics.go",
					Name:     "latency_seconds",
					FullName: "latency_seconds",
					Type:     "histogram",
				},
				{
					FilePath: "metrics.go",
					Name:     "duration_seconds",
					FullName: "duration_seconds",
					Type:     "summary",
				},
				{
					FilePath: "default_import.go",
					Name:     "default_import",
					FullName: "default_import",
					Type:     "counter",
				},
				{
					FilePath: "standalone_comment.go",
					Name:     "standalone_metric",
					Help:     "Tracks standalone operations.",
					FullName: "standalone_metric",
					Type:     "counter",
				},
				{
					FilePath:  "metrics.go",
					Namespace: "proxy_peer",
					Subsystem: "client",
					Name:      "dial_error_total",
					Help:      "Total number of errors encountered dialing peer proxies.",
					FullName:  "proxy_peer_client_dial_error_total",
					Type:      "counter",
				},
				{
					FilePath:  "metrics.go",
					Namespace: "proxy_peer",
					Subsystem: "client",
					Name:      "connections",
					Help:      "Number of currently opened connections to proxy peer servers.",
					FullName:  "proxy_peer_client_connections",
					Type:      "gauge",
				},
				{
					FilePath:  "metrics.go",
					Namespace: "teleport",
					Subsystem: "cache",
					Name:      "service_operations",
					Help:      "Operations for",
					FullName:  "teleport_cache_service_operations",
					Type:      "counter",
				},
			},
		},
		{
			description:    "module root not found",
			files:          map[string]string{},
			errorSubstring: "finding module root",
		},
		{
			description: "invalid source",
			files: map[string]string{
				"go.mod":     "module example.com/project\n",
				"invalid.go": "package project\nfunc invalid(",
			},
			errorSubstring: "loading Go source files",
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			moduleRoot := t.TempDir()
			for relativePath, contents := range tc.files {
				writeTestFile(t, moduleRoot, relativePath, contents)
			}

			rootPath := moduleRoot
			if tc.scanRoot != "" {
				rootPath = filepath.Join(moduleRoot, tc.scanRoot)
			}
			metrics, err := CollectMetrics("example.com/project", rootPath)
			if tc.errorSubstring != "" {
				require.ErrorContains(t, err, tc.errorSubstring)
				return
			}
			require.NoError(t, err)

			assert.ElementsMatch(t, tc.expectedMetrics, metrics)
		})
	}
}

func TestFindModuleRoot(t *testing.T) {
	moduleRoot := t.TempDir()
	writeTestFile(t, moduleRoot, "go.mod", "module example.com/project\n")
	filePath := writeTestFile(t, moduleRoot, "nested/pkg/source.go", "package pkg\n")

	cases := []struct {
		description string
		startPath   string
		expected    string
	}{
		{
			description: "from directory",
			startPath:   filepath.Dir(filePath),
			expected:    moduleRoot,
		},
		{
			description: "from file",
			startPath:   filePath,
			expected:    moduleRoot,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			actual, err := findModuleRoot(tc.startPath)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCollectModuleConsts(t *testing.T) {
	moduleRoot := t.TempDir()
	writeTestFile(t, moduleRoot, "go.mod", "module example.com/project\n")
	writeTestFile(t, moduleRoot, "constants.go", `
	package project

    type metricVersion int
    
    const (
    	Namespace = "teleport"
    	NamespaceAlias = Namespace
    	Version = metricVersion(3)
    	Enabled = true
    )
    `)
	writeTestFile(t, moduleRoot, "pkg/constants.go", `
	package pkg
	
    const Subsystem = "proxy"
    `)
	writeTestFile(t, moduleRoot, "pkg/constants_test.go", `
	package pkg

    const TestOnly = "ignored"
`)
	writeTestFile(t, moduleRoot, "nested/go.mod", "module example.com/nested\n")
	writeTestFile(t, moduleRoot, "nested/constants.go", "package nested\nconst Nested = \"ignored\"\n")

	constants, err := collectModuleConsts("example.com/project", moduleRoot)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"Namespace":      "teleport",
		"NamespaceAlias": "teleport",
		"Version":        int64(3),
	}, constants["example.com/project"])
	assert.Equal(t, map[string]any{
		"Subsystem": "proxy",
	}, constants["example.com/project/pkg"])
	assert.NotContains(t, constants, "example.com/project/nested")
}

func TestResolveConstantValue(t *testing.T) {
	constants := map[string]any{
		"Namespace": "teleport",
	}
	cases := []struct {
		description string
		expression  string
		expected    any
		ok          bool
	}{
		{
			description: "string",
			expression:  `"teleport"`,
			expected:    "teleport",
			ok:          true,
		},
		{
			description: "raw string",
			expression:  "`teleport`",
			expected:    "teleport",
			ok:          true,
		},
		{
			description: "integer",
			expression:  "0x10",
			expected:    int64(16),
			ok:          true,
		},
		{
			description: "identifier",
			expression:  "Namespace",
			expected:    "teleport",
			ok:          true,
		},
		{
			description: "conversion",
			expression:  "metricVersion(2)",
			expected:    int64(2),
			ok:          true,
		},
		{
			description: "concatenated strings",
			expression:  `"tele" + "port"`,
			expected:    "teleport",
			ok:          true,
		},
		{
			description: "concatenated identifier",
			expression:  `Namespace + "_auth"`,
			expected:    "teleport_auth",
			ok:          true,
		},
		{
			description: "mixed concatenation",
			expression:  `"teleport" + 1`,
			ok:          false,
		},
		{
			description: "unknown identifier",
			expression:  "Unknown",
			ok:          false,
		},
		{
			description: "unsupported literal",
			expression:  "1.5",
			ok:          false,
		},
		{
			description: "multi-argument call",
			expression:  "format(1, 2)",
			ok:          false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			expression, err := parser.ParseExpr(tc.expression)
			require.NoError(t, err)
			actual, ok := resolveConstantValue(expression, constants)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestExpressionResolver(t *testing.T) {
	const (
		prefix      = "example.com/project"
		packagePath = prefix + "/metrics"
	)
	imports := map[string]string{
		"fmt":      "fmt",
		"project":  prefix,
		"registry": prefix + "/lib/observability/metrics",
		"other":    "example.com/other",
	}
	constants := map[string]map[string]any{
		prefix: {
			"MetricNamespace": "teleport",
			"Version":         int64(2),
		},
		packagePath: {
			"metricsSubsystem": "auth",
		},
	}

	file, err := parser.ParseFile(token.NewFileSet(), "source.go", `package metrics
func register(reg *registry.Registry, unrelated *other.Registry) {}
`, 0)
	require.NoError(t, err)
	function := file.Decls[0].(*ast.FuncDecl)
	resolver := newExpressionResolver(packagePath, imports, constants).forFunction(function, prefix, "teleport")
	resolver.locals["metricName"] = "requests"

	cases := []struct {
		description string
		expression  string
		expected    string
		ok          bool
	}{
		{description: "literal", expression: `"requests"`, expected: "requests", ok: true},
		{description: "local value", expression: "metricName", expected: "requests", ok: true},
		{description: "package constant", expression: "metricsSubsystem", expected: "auth", ok: true},
		{description: "imported constant", expression: "project.MetricNamespace", expected: "teleport", ok: true},
		{description: "concatenation", expression: `project.MetricNamespace + "_" + metricName`, expected: "teleport_requests", ok: true},
		{description: "sprintf", expression: `fmt.Sprintf("%s_v%d", metricName, project.Version)`, expected: "requests_v2", ok: true},
		{description: "sprintf unresolved argument", expression: `fmt.Sprintf("requests for %s", runtimeValue())`, expected: "requests for", ok: true},
		{description: "registry namespace", expression: "reg.Namespace()", expected: "teleport", ok: true},
		{description: "registry subsystem", expression: "reg.Subsystem()", expected: "auth", ok: true},
		{description: "unrelated registry", expression: "unrelated.Namespace()", ok: false},
		{description: "unknown value", expression: "unknown", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			expression, err := parser.ParseExpr(tc.expression)
			require.NoError(t, err)
			actual, ok := resolver.resolveString(expression)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestRecordLocalValues(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", `package metrics
func register() {
	metricName := "requests"
	metricName = "operations"
	metricName = runtimeValue()
}
`, 0)
	require.NoError(t, err)
	function := file.Decls[0].(*ast.FuncDecl)
	resolver := newExpressionResolver("example.com/project/metrics", nil, nil)

	resolver.recordLocalValues(function.Body.List[0])
	assert.Equal(t, "requests", resolver.locals["metricName"])
	resolver.recordLocalValues(function.Body.List[1])
	assert.Equal(t, "operations", resolver.locals["metricName"])
	resolver.recordLocalValues(function.Body.List[2])
	assert.NotContains(t, resolver.locals, "metricName")
}

func TestTruncateBeforeVerb(t *testing.T) {
	cases := []struct {
		description string
		format      string
		verbIndex   int
		expected    string
	}{
		{description: "first verb", format: "prefix %s suffix", verbIndex: 0, expected: "prefix "},
		{description: "second verb", format: "%s prefix %d suffix", verbIndex: 1, expected: "%s prefix "},
		{description: "escaped percent", format: "100%% prefix %s", verbIndex: 0, expected: "100%% prefix "},
		{description: "missing verb", format: "prefix", verbIndex: 0, expected: "prefix"},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, truncateBeforeVerb(tc.format, tc.verbIndex))
		})
	}
}

func TestNamedImports(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    map[string]string
	}{
		{
			description: "single-line named import",
			input: `package metrics

                import prom "github.com/prometheus/client_golang/prometheus"
                `,
			expected: map[string]string{
				"prom": "github.com/prometheus/client_golang/prometheus",
			},
		},
		{
			description: "multi-line imports",
			input: `package metrics

                import (
                	"fmt"
                	prom "github.com/prometheus/client_golang/prometheus"
                )
                `,
			expected: map[string]string{
				"fmt":  "fmt",
				"prom": "github.com/prometheus/client_golang/prometheus",
			},
		},
		{
			description: "blank import excluded",
			input: `package metrics

                    import (
                    	_ "example.com/blank"
                    	 prom "github.com/prometheus/client_golang/prometheus"
                    )
                    `,
			expected: map[string]string{
				"prom": "github.com/prometheus/client_golang/prometheus",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "source.go", tc.input, parser.ImportsOnly)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, namedImports(file))
		})
	}
}

func TestExtractMetricData(t *testing.T) {
	expression, err := parser.ParseExpr(`prom.CounterOpts{
	Namespace: "teleport",
	Subsystem: unresolved,
	Name: "requests",
	Help: "Number of requests.",
	ConstLabels: labels,
}`)
	require.NoError(t, err)
	literal, ok := expression.(*ast.CompositeLit)
	require.True(t, ok)

	// Define a minimal stub that can resolve string literals.
	resolve := func(expression ast.Expr) (string, bool) {
		literal, ok := expression.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return "", false
		}
		return literal.Value[1 : len(literal.Value)-1], true
	}
	namespace, subsystem, name, help := extractMetricData(literal, resolve)
	assert.Equal(t, "teleport", namespace)
	assert.Empty(t, subsystem)
	assert.Equal(t, "requests", name)
	assert.Equal(t, "Number of requests.", help)
}

func TestAssembleFullName(t *testing.T) {
	cases := []struct {
		description string
		namespace   string
		subsystem   string
		name        string
		expected    string
	}{
		{description: "all components", namespace: "teleport", subsystem: "auth", name: "requests", expected: "teleport_auth_requests"},
		{description: "without namespace", subsystem: "auth", name: "requests", expected: "auth_requests"},
		{description: "without subsystem", namespace: "teleport", name: "requests", expected: "teleport_requests"},
		{description: "name only", name: "requests", expected: "requests"},
		{description: "empty", expected: ""},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, assembleFullName(tc.namespace, tc.subsystem, tc.name))
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
