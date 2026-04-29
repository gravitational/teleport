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
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

func runLint(ctx context.Context, charts []Chart, rootDir string) error {
	// Preflight check ot make sure yamllint is installed
	if err := checkDependencies(yamllintBinName, helmBinName); err != nil {
		return trace.Wrap(err, "preflight checks")
	}

	configPath := filepath.Join(rootDir, yamlLintConfigPath)

	for _, chart := range charts {
		if chart.IsLibrary {
			continue
		}
		fmt.Println("Running lint for chart:", chart.Name)
		valuesDir := filepath.Join(chart.Path, ".lint")
		content, err := os.ReadDir(valuesDir)
		if err != nil && !trace.IsNotFound(trace.ConvertSystemError(err)) {
			return trace.Wrap(err, "reading values directory for %s", chart.Path)
		}

		values := make([]os.DirEntry, 0, len(content))
		for _, file := range content {
			if file.IsDir() {
				log.Println("Skipping directory", file.Name())
				continue
			}
			ext := filepath.Ext(file.Name())
			if ext != ".yaml" && ext != ".yml" {
				log.Printf("Skipping non-yaml file %s", file.Name())
				continue
			}
			values = append(values, file)
		}

		// If the chart has no lint directory or if it's empty, we lint with the default values.
		if len(values) == 0 {
			if err := lint(ctx, "", configPath, chart); err != nil {
				return trace.Wrap(err, "linting chart %s with default values", chart.Path)
			}
		}

		for _, file := range values {
			if err := lint(ctx, filepath.Join(valuesDir, file.Name()), configPath, chart); err != nil {
				return trace.Wrap(err, "linting chart %s", chart.Path)
			}
		}
	}

	fmt.Println(" ✅ Charts successfully linted.")
	return nil
}

// lint runs all the lint operations on a chart for a given value file.
// If the value file path is empty, the chart is linted with its default values.
func lint(ctx context.Context, valuesPath, configPath string, chart Chart) error {
	// Yamllint the values
	if valuesPath != "" {
		if stdout, stderr, err := run(ctx, yamllintBinName, "-c", configPath, valuesPath); err != nil {
			fmt.Printf(" ❌ yamllint values %q failed\n", valuesPath)
			// yamllint seems to output to stdout
			fmt.Println(string(stdout))
			fmt.Println(string(stderr))
			return trace.Wrap(err, "linting values %q", valuesPath)
		}
	}

	// Helm lint
	args := []string{
		"lint", "--quiet", "--strict", chart.Path,
	}
	if valuesPath != "" {
		args = append(args, "-f", valuesPath)
	}
	if stdout, stderr, err := run(ctx, helmBinName, args...); err != nil {
		fmt.Printf(" ❌ Helm linting chart %q failed with values %q\n", chart.Name, valuesPath)
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return trace.Wrap(err, "linting with values %q", valuesPath)
	}

	// Render the manifests
	tmpDest, err := os.CreateTemp("", "*-out.yaml")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		tmpDest.Close()
		os.Remove(tmpDest.Name())
	}()

	args = []string{"template", "test", chart.Path}
	if valuesPath != "" {
		args = append(args, "-f", valuesPath)
	}
	rendered, stderr, err := run(ctx, helmBinName, args...)
	if err != nil {
		fmt.Println(string(stderr))
		return trace.Wrap(err, "rendering templates for values %q", valuesPath)
	}
	if _, err := tmpDest.Write(rendered); err != nil {
		return trace.ConvertSystemError(err)
	}

	// Yammllint the manifests
	if stdout, stderr, err := run(ctx, yamllintBinName, "-c", configPath, tmpDest.Name()); err != nil {
		fmt.Printf(" ❌ yamllint rendered chart %q with values %q failed\n", chart.Name, valuesPath)
		// We output the linted template to stdout with line numbers to make finding and fixing the error easier.
		fmt.Println(" 🔎 Linted templates:")
		printWithLineNumbers(rendered)
		fmt.Println()
		fmt.Println(" ⚠️ Linting errors:")
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return trace.Wrap(err, "linting rendered templates for values %q", valuesPath)
	}

	return nil
}

func printWithLineNumbers(stdout []byte) {
	lines := strings.Split(string(stdout), "\n")
	count := len(lines)
	numDigit := len(strconv.FormatInt(int64(count), 10))

	for i := 0; i < count; i++ {
		fmt.Printf("%*d |  %s\n", numDigit+1, i+1, lines[i])
	}
}
