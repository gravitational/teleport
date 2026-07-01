package tmig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/config"
	"github.com/gravitational/teleport/lib/tmig/mapping"
	"github.com/gravitational/teleport/lib/tmig/report"
	"github.com/gravitational/teleport/lib/tmig/rewrite"
	"github.com/gravitational/teleport/lib/tmig/runstate"
)

// HostData aggregates everything known about a host before classification.
type HostData struct {
	UUID               string
	Hostname           string
	Labels             map[string]string
	Recon              classify.ReconResult
	DiscoveryEnrolled  bool
	CoveringTokenFound bool
}

// RunPreflight orchestrates the preflight stage.
// For now it uses fixture/stub data; real cluster + SSH connectivity
// will be wired in when those packages are fully connected.
func RunPreflight(ctx context.Context, cfg *config.Config, outputDir string, format string, resume bool) error {
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	state, err := runstate.New(outputDir)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	_ = state // used when real inventory feeds in

	// TODO: connect to SOURCE and TARGET clusters
	// TODO: run preflight checks (identity pin, scopes enabled, capability)
	// TODO: enumerate agents and probe via SSH
	// For now, return an error indicating the pipeline needs real connectivity
	return fmt.Errorf("preflight requires cluster connectivity (not yet wired)")
}

// RunInventory orchestrates the inventory stage.
func RunInventory(ctx context.Context, cfg *config.Config, format string) error {
	// TODO: connect to each source, enumerate, probe, resolve mappings
	return fmt.Errorf("inventory requires cluster connectivity (not yet wired)")
}

// BuildPreflightReport is the pure core of preflight: given all gathered data,
// produce the report. This is fully testable without cluster connectivity.
func BuildPreflightReport(
	hosts []HostData,
	mappings []config.Mapping,
	scopableMethods []string,
	scopableRoles map[string]bool,
	rewriter rewrite.ConfigRewriter,
	targetProxy string,
	meta report.ReportMeta,
) (*report.Report, error) {
	ctx := context.Background()
	var inputs []report.HostInput

	for _, h := range hosts {
		mr := mapping.Resolve(mapping.HostLabels(h.Labels), mappings)
		ci := classify.ClassifyInput{
			Recon:              h.Recon,
			Orphan:             mr.Orphan,
			ScopedTarget:       !mr.Orphan && mr.Matched != nil && mr.Matched.Scope != "",
			ScopableMethods:    scopableMethods,
			ScopableRoles:      scopableRoles,
			DiscoveryEnrolled:  h.DiscoveryEnrolled,
			CoveringTokenFound: h.CoveringTokenFound,
		}
		co := classify.Classify(ci)

		var mappingStr string
		if mr.Orphan {
			mappingStr = "(orphan)"
		} else if mr.Matched != nil {
			mappingStr = mr.Matched.Scope
		} else if mr.Conflict != nil {
			mappingStr = "(conflict)"
		}

		var diff string
		var mode string
		if !mr.Orphan && mr.Matched != nil && co.Verdict != classify.VerdictManual {
			result, err := rewriter.Preview(ctx, h.Recon, *mr.Matched, targetProxy)
			if err == nil {
				diff = result.Diff
				mode = result.Mode
			}
		}

		inputs = append(inputs, report.HostInput{
			HostUUID:       h.UUID,
			Hostname:       h.Hostname,
			Mapping:        mappingStr,
			JoinMethod:     h.Recon.JoinMethod,
			Services:       h.Recon.Services,
			ConfigDiff:     diff,
			RewriteMode:    mode,
			ClassifyOutput: co,
		})
	}

	return report.Build(inputs, meta), nil
}

// WriteReportFiles writes the JSON and HTML report files to the output directory.
func WriteReportFiles(rpt *report.Report, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// JSON
	jsonPath := filepath.Join(outputDir, "readiness.json")
	f, err := os.Create(jsonPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rpt); err != nil {
		return err
	}

	// HTML
	htmlPath := filepath.Join(outputDir, "readiness.html")
	hf, err := os.Create(htmlPath)
	if err != nil {
		return err
	}
	defer hf.Close()
	if err := report.RenderHTML(rpt, hf); err != nil {
		return err
	}

	return nil
}
