// Package inventory provides types for agents from the cluster inventory
// and a BuildSummary function that computes fleet distribution counts.
package inventory

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/cluster"
)

// Agent represents one agent from the cluster inventory.
type Agent struct {
	UUID       string
	Hostname   string
	Labels     map[string]string
	Services   []string
	Version    string
	JoinMethod string
}

// Summary provides fleet distribution counts (the NVIDIA ask).
type Summary struct {
	Total         int
	Reachable     int
	Unreachable   int
	ByInstallKind map[classify.InstallKind]int
	ByJoinMethod  map[string]int
	ByVersion     map[string]int
	ByServiceMix  map[string]int // key = sorted comma-joined services
}

// BuildSummary computes distribution counts from agents and their recon results.
func BuildSummary(agents []Agent, recon map[string]classify.ReconResult) Summary {
	s := Summary{
		ByInstallKind: make(map[classify.InstallKind]int),
		ByJoinMethod:  make(map[string]int),
		ByVersion:     make(map[string]int),
		ByServiceMix:  make(map[string]int),
	}
	for _, a := range agents {
		s.Total++
		s.ByVersion[a.Version]++
		r, ok := recon[a.UUID]
		if ok && r.Reachable {
			s.Reachable++
			s.ByInstallKind[r.InstallKind]++
			if r.JoinMethod != "" {
				s.ByJoinMethod[r.JoinMethod]++
			}
		} else {
			s.Unreachable++
		}
	}
	return s
}

// Enumerate lists all agents from the cluster inventory (paginated).
// TODO: implement using the Teleport client's inventory API.
func Enumerate(ctx context.Context, client *cluster.Client) ([]Agent, error) {
	return nil, fmt.Errorf("inventory enumeration not yet implemented")
}
