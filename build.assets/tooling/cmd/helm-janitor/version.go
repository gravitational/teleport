package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

func updateVersion(ctx context.Context, version string) error {
	for _, chart := range charts {
		if err := updateChartVersion(ctx, chart, version); err != nil {
			return trace.Wrap(err, "updating version of chart %q", chart.Name)
		}
	}
	return nil
}

var versionRegex = regexp.MustCompile(`\.version: .*`)

func updateChartVersion(ctx context.Context, chart Chart, version string) error {
	version = strings.TrimPrefix(version, "v")
	chartYaml, err := os.ReadFile(filepath.Join(chart.Path, "Chart.yaml"))
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "reading Chart.yaml")
	}
	newChartYaml := versionRegex.ReplaceAll(chartYaml, []byte(fmt.Sprintf(`.version: &version %q`, version)))
	if bytes.Equal(chartYaml, newChartYaml) {
		fmt.Printf("Warning: Chart.yaml unchanged: %q\n", chart.Path)
	}
	if err := os.WriteFile(filepath.Join(chart.Path, "Chart.yaml"), newChartYaml, 0644); err != nil {
		return trace.Wrap(err, "writing Chart.yaml")
	}

	return nil
}
