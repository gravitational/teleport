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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

const (
	yamllintBinName    = "yamllint"
	helmBinName        = "helm"
	yamlLintConfigPath = "examples/chart/.lint-config.yaml"
)

func checkDependencies(names ...string) error {
	for _, name := range names {
		_, err := exec.LookPath(name)
		if err != nil {
			return trace.NotFound("%s not found in $PATH", name)
		}
	}
	return nil
}

func run(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.Bytes(), stderr.Bytes(), trace.Wrap(err, "command %s exited with status %d", command, exitErr.ExitCode())
		}
		return stdout.Bytes(), stderr.Bytes(), trace.Wrap(err)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func chartsWithPath(rootDir string) []Chart {
	if rootDir == "" {
		rootDir = "."
	}
	pathedCharts := make([]Chart, len(charts))
	for i, chart := range charts {
		var path, referencePath string
		if chart.Path != "" {
			path = filepath.Join(rootDir, chart.Path)
		}
		if chart.ReferencePath != "" {
			referencePath = filepath.Join(rootDir, chart.ReferencePath)
		}
		pathedCharts[i] = Chart{
			Name:          chart.Name,
			Path:          path,
			ReferencePath: referencePath,
			IsLibrary:     chart.IsLibrary,
		}
	}
	return pathedCharts
}

func selectCharts(chartNames string, rootDir string) ([]Chart, error) {
	charts := chartsWithPath(rootDir)
	if chartNames == "" {
		return charts, nil
	}

	validNameSet := make(map[string]struct{})
	for _, chart := range charts {
		validNameSet[chart.Name] = struct{}{}
	}

	selectedNames := strings.Split(chartNames, ",")
	selectedNameSet := make(map[string]struct{})
	for _, name := range selectedNames {
		if _, ok := validNameSet[name]; !ok {
			return nil, trace.NotFound("unknown chart name: %s", name)
		}
		selectedNameSet[strings.TrimSpace(name)] = struct{}{}
	}

	var selected []Chart
	for _, chart := range charts {
		if _, ok := selectedNameSet[chart.Name]; ok {
			selected = append(selected, chart)
		}
	}

	return selected, nil
}
