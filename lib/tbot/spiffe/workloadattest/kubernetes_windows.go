//go:build windows

package workloadattest

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
)

// WindowsKubernetesAttestor is the windows stub for KubernetesAttestor.
type WindowsKubernetesAttestor struct {
}

func (a WindowsKubernetesAttestor) Attest(_ context.Context, _ int) (KubernetesAttestation, error) {
	return KubernetesAttestation{}, trace.NotImplemented("kubernetes attestation is not supported on windows")
}

// NewKubernetesAttestor creates a new KubernetesAttestor.
func NewKubernetesAttestor(_ KubernetesAttestorConfig, _ *slog.Logger) *WindowsKubernetesAttestor {
	return &WindowsKubernetesAttestor{}
}
