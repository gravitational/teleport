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

func runLint(ctx context.Context, charts []Chart) error {
	// Preflight check ot make sure yamllint is installed
	if err := checkDependencies(yamllintBinName, helmBinName); err != nil {
		return trace.Wrap(err, "preflight checks")
	}

	for _, chart := range charts {
		fmt.Println("Running lint for chart:", chart.Name)
		valuesDir := filepath.Join(chart.Path, ".lint")
		content, err := os.ReadDir(valuesDir)
		if err != nil && !trace.IsNotFound(trace.ConvertSystemError(err)) {
			return trace.Wrap(err, "reading values directory for %s", chart.Path)
		}

		// If the chart has no lint directory or if it's empty, we lint with the default values.
		if len(content) == 0 {
			return trace.Wrap(lint(ctx, "", chart), "linting chart %s", chart.Path)
		}

		for _, file := range content {
			if file.IsDir() {
				continue
			}
			ext := filepath.Ext(file.Name())
			if ext != ".yaml" && ext != ".yml" {
				log.Printf("Skipping non-yaml file %s", file.Name())
				continue
			}
			if err := lint(ctx, filepath.Join(valuesDir, file.Name()), chart); err != nil {
				return trace.Wrap(err, "linting chart %s", chart.Path)
			}
		}
	}
	return nil
}

// lint runs all the lint operations on a chart for a given value file.
// If the value file path is empty, the chart is linted with its default values.
func lint(ctx context.Context, valuesPath string, chart Chart) error {
	// Yamllint the values
	if valuesPath != "" {
		if stdout, stderr, err := run(ctx, yamllintBinName, "-c", yamlLintConfigPath, valuesPath); err != nil {
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
	if stdout, stderr, err := run(ctx, yamllintBinName, "-c", yamlLintConfigPath, tmpDest.Name()); err != nil {
		// yamllint seems to output to stdout
		fmt.Println("Linted templates:")
		printWithLineNumbers(rendered)
		fmt.Println()
		fmt.Println("Linting errors:")
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
