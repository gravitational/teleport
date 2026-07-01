package recon

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// Compile-time interface check.
var _ Prober = (*SSHProber)(nil)

// SSHProber probes hosts via Teleport's SSH proxy path.
type SSHProber struct {
	Login string
	// TODO: add Teleport client for SSH execution
}

// Probe connects to the host via SSH and runs the recon script.
func (p *SSHProber) Probe(ctx context.Context, hostUUID string, hostname string) (classify.ReconResult, error) {
	// TODO: implement real SSH via Teleport proxy
	// This will use lib/client to establish an SSH session through the SOURCE cluster,
	// run RenderScript(), capture stdout, and call ParseOutput().
	return classify.ReconResult{}, fmt.Errorf("SSH probe not yet implemented for %s", hostname)
}
