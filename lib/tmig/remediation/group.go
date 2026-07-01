package remediation

import "github.com/gravitational/teleport/lib/tmig/report"

// HostRemediation ties a host to a remediation key for grouping.
type HostRemediation struct {
	Hostname string
	Key      string      // de-duplication key (e.g. "iam-883900000000-Node-/dgxc/team-a")
	Params   TokenParams // for IaC remediations
}

// Group de-duplicates remediations by key and collects covered hosts.
// N blocked hosts collapse into M distinct IaC actions where M <= N.
func Group(hosts []HostRemediation) []report.Remediation {
	type group struct {
		params TokenParams
		hosts  []string
		order  int
	}
	groups := make(map[string]*group)
	var order int
	for _, h := range hosts {
		g, exists := groups[h.Key]
		if !exists {
			g = &group{params: h.Params, order: order}
			groups[h.Key] = g
			order++
		}
		g.hosts = append(g.hosts, h.Hostname)
	}

	result := make([]report.Remediation, 0, len(groups))
	// Sort by insertion order
	sorted := make([]*group, len(groups))
	for _, g := range groups {
		sorted[g.order] = g
	}
	for _, g := range sorted {
		if g == nil {
			continue
		}
		rem := EmitScopedToken(g.params)
		rem.HostsCovered = g.hosts
		result = append(result, rem)
	}
	return result
}
