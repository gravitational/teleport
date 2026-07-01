// Package mapping implements resolution of hosts to migration mappings by label selectors.
package mapping

import "github.com/gravitational/teleport/lib/tmig/config"

// HostLabels is the label set from a host.
type HostLabels map[string]string

// Result of resolving a host against mappings.
type Result struct {
	Matched  *config.Mapping  // non-nil if exactly one match
	Orphan   bool             // true if zero matches
	Conflict []config.Mapping // non-nil if two+ matches
}

// Resolve evaluates a host's labels against all mappings.
// Zero matches = orphan. One match = assigned. Two+ = conflict (config bug).
func Resolve(host HostLabels, mappings []config.Mapping) Result {
	var matches []config.Mapping
	for i := range mappings {
		if selectorMatches(host, mappings[i].Selector) {
			matches = append(matches, mappings[i])
		}
	}
	switch len(matches) {
	case 0:
		return Result{Orphan: true}
	case 1:
		return Result{Matched: &matches[0]}
	default:
		return Result{Conflict: matches}
	}
}

// selectorMatches returns true if the host has all key=value pairs in the selector.
func selectorMatches(host HostLabels, selector map[string]string) bool {
	for k, v := range selector {
		if host[k] != v {
			return false
		}
	}
	return true
}
