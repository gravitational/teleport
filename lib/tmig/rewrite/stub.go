package rewrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/config"
	"github.com/gravitational/teleport/lib/tmig/redact"
)

// StubRewriter returns realistic fixture diffs.
// TODO: Replace with OperatorRewriter that reads teleport.yaml over SSH,
// edits in-process, and validates through lib/config loader.
type StubRewriter struct{}

// NewStubRewriter creates a new StubRewriter.
func NewStubRewriter() *StubRewriter {
	return &StubRewriter{}
}

// Preview generates a realistic fixture diff showing what config changes would
// be applied for the given host and mapping.
func (s *StubRewriter) Preview(ctx context.Context, host classify.ReconResult, mapping config.Mapping, targetProxy string) (RewriteResult, error) {
	var diff strings.Builder
	diff.WriteString("  teleport:\n")
	diff.WriteString("-   proxy_server: <source-proxy>\n")
	diff.WriteString(fmt.Sprintf("+   proxy_server: %s\n", targetProxy))

	if host.JoinMethod == "token" {
		diff.WriteString("+   join_params:\n")
		diff.WriteString("+     method: token\n")
		diff.WriteString("+     token_name: <redacted>\n")
		diff.WriteString("-   auth_token: <redacted>\n")
	} else {
		diff.WriteString("    join_params:\n")
		diff.WriteString(fmt.Sprintf("      method: %s\n", host.JoinMethod))
		diff.WriteString("-     token_name: <source-token>\n")
		diff.WriteString("+     token_name: <target-scoped-token>\n")
	}

	diff.WriteString(fmt.Sprintf("+   data_dir: /var/lib/teleport_%s\n", mapping.InstallSuffix))

	for _, svc := range host.Services {
		if svc == "auth_service" {
			diff.WriteString("  auth_service:\n")
			diff.WriteString("-   enabled: true\n")
			diff.WriteString("+   enabled: false\n")
		}
	}

	redacted := redact.Config(diff.String())
	return RewriteResult{
		Diff:  redacted,
		Mode:  "stub",
		Valid: true,
	}, nil
}
