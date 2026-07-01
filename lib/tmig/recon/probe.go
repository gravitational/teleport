package recon

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// Prober runs the SSH recon probe against a host.
type Prober interface {
	Probe(ctx context.Context, hostUUID string, hostname string) (classify.ReconResult, error)
}

// ParseOutput parses the key=value output from the probe script.
func ParseOutput(raw string) (classify.ReconResult, error) {
	result := classify.ReconResult{Reachable: true}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "TMIG_") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "TMIG_OS":
			result.OS = val
		case "TMIG_SYSTEMD":
			result.HasSystemd = val == "true"
		case "TMIG_TELEPORT_UPDATE":
			result.HasTeleportUpdate = val == "true"
		case "TMIG_CONFIG_PATH":
			result.ConfigPath = val
		case "TMIG_CONFIG_READABLE":
			result.ConfigReadable = val == "true"
		case "TMIG_ROOT":
			result.RootPath = val == "true"
		case "TMIG_JOIN_METHOD":
			result.JoinMethod = val
		case "TMIG_SERVICES":
			if val != "" {
				result.Services = strings.Split(val, ",")
			}
		case "TMIG_LISTEN_ADDRS":
			if val != "" {
				result.ListenAddrs = strings.Split(val, ",")
			}
		case "TMIG_BINARY_VERSION":
			result.BinaryVersion = val
		case "TMIG_INSTALL_KIND":
			result.InstallKind = classify.InstallKind(val)
		}
	}
	if result.OS == "" {
		return result, fmt.Errorf("probe output missing TMIG_OS")
	}
	return result, nil
}
