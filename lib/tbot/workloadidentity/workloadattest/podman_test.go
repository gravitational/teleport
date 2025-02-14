package workloadattest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPodmanAttestorConfig_CheckAndSetDefaults(t *testing.T) {
	validCases := map[string]PodmanAttestorConfig{
		"attestor disabled": {Enabled: false, Addr: ""},
		"unix socket":       {Enabled: true, Addr: "unix:///path/to/socket"},
	}
	for name, cfg := range validCases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, cfg.CheckAndSetDefaults())
		})
	}

	invalidCases := map[string]struct {
		cfg PodmanAttestorConfig
		err string
	}{
		"no addr": {
			cfg: PodmanAttestorConfig{Enabled: true, Addr: ""},
			err: "is required",
		},
		"not a UDS": {
			cfg: PodmanAttestorConfig{Enabled: true, Addr: "https://localhost:1234"},
			err: "must be in the form `unix://path/to/socket`",
		},
		"missing path": {
			cfg: PodmanAttestorConfig{Enabled: true, Addr: "unix://"},
			err: "must be in the form `unix://path/to/socket`",
		},
		"missing leading slash": {
			cfg: PodmanAttestorConfig{Enabled: true, Addr: "unix://path/to/file"},
			err: "host segment must be empty, did you forget a leading slash in the socket path? (i.e. `unix:///path/to/file`)",
		},
	}
	for name, tc := range invalidCases {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, tc.cfg.CheckAndSetDefaults(), tc.err)
		})
	}
}
