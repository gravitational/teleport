// Package rewrite implements config rewriting for Teleport scope migrations.
package rewrite

import (
	"context"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/config"
)

// ConfigRewriter produces a preview of the config changes for a host.
// Two implementations:
// - StubRewriter: returns realistic fixture diffs (current default)
// - OperatorRewriter: reads teleport.yaml over SSH, edits in-process,
//   validates through lib/config loader (replaces stub when implemented)
// - NodeLocalRewriter: shells out to `teleport reconfigure` on the host
//   (future, when the command ships)
type ConfigRewriter interface {
	Preview(ctx context.Context, host classify.ReconResult, mapping config.Mapping, targetProxy string) (RewriteResult, error)
}

// RewriteResult is the output of a config rewrite preview.
type RewriteResult struct {
	Diff   string   // redacted unified diff
	Mode   string   // "stub" | "operator-side" | "node-local"
	Valid  bool     // true if the rewritten config passes validation
	Errors []string // validation errors, if any
}
