/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

func updateVersion(ctx context.Context, version string, charts []Chart) error {
	for _, chart := range charts {
		if err := updateChartVersion(ctx, chart, version); err != nil {
			return trace.Wrap(err, "updating version of chart %q", chart.Name)
		}
	}
	fmt.Printf(" ✅ Version updated to %s\n", version)
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
		fmt.Printf(" ⚠️ Warning: Chart.yaml unchanged: %q\n", chart.Path)
	}
	if err := os.WriteFile(filepath.Join(chart.Path, "Chart.yaml"), newChartYaml, 0644); err != nil {
		return trace.Wrap(err, "writing Chart.yaml")
	}

	return nil
}
