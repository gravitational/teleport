package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

func runTest(ctx context.Context, charts []Chart, updateSnapshots bool) error {
	if err := checkDependencies(helmBinName); err != nil {
		return trace.Wrap(err, "preflight checks")
	}

	for _, chart := range charts {
		if chart.IsLibrary {
			continue
		}
		if err := testHelm(ctx, chart, updateSnapshots); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Println(" ✅ All tests succeeded")
	return nil
}

func testHelm(ctx context.Context, chart Chart, updateSnapshots bool) error {
	fmt.Println("Running tests for chart:", chart.Name)
	args := []string{"unittest", "-3", "--with-subchart=false", chart.Path}
	if updateSnapshots {
		args = append(args, "-u")
	}
	// We log the test command so it's easier for a developer to copy it and re-run to target a failing test.
	fmt.Printf("▶️ %s %s\n", helmBinName, strings.Join(args, " "))
	stdout, stderr, err := run(ctx, helmBinName, args...)
	if err != nil {
		fmt.Printf(" ❌ Helm unit tests failed for chart %q", chart.Path)
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return trace.Wrap(err, "testing chart %q", chart.Path)
	}
	return nil
}
