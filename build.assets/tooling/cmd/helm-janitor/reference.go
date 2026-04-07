package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/build.assets/tooling/lib/helm"
	"github.com/gravitational/trace"
)

func runReference(ctx context.Context, charts []Chart, check bool) error {
	for _, chart := range charts {
		if chart.ReferencePath == "" {
			// teleport-cluster's reference is not yet rendered
			continue
		}
		fmt.Printf("Rendering reference for chart %s\n", chart.Path)
		ref, err := helm.RenderReference(chart.Path)
		if err != nil {
			return trace.Wrap(err, "rendering chart reference for %q", chart.Path)
		}
		if check {
			existing, err := os.ReadFile(chart.ReferencePath)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			if !bytes.Equal(existing, ref) {
				fmt.Printf(" ❌ Out-of-sync reference for chart %q.\n", chart.Name)
				fmt.Println("Please run `make helm-render-chart-ref`")
				fmt.Println()
				fmt.Println(cmp.Diff(string(existing), string(ref)))
				return trace.CompareFailed("reference is out of date for chart %q", chart.Path)
			}
			continue
		}
		if err := os.WriteFile(chart.ReferencePath, ref, 0644); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	if check {
		fmt.Println(" ✅ All references are up-to-date")
	} else {
		fmt.Println(" ✅ All references rendered")
	}
	return nil
}
