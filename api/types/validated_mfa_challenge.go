package types

import (
	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
)

// ValidatedMFAChallengeFilter is a filter for ValidatedMFAChallenge resources.
type ValidatedMFAChallengeFilter struct {
	TargetCluster string
}

func (f *ValidatedMFAChallengeFilter) IntoMap() map[string]string {
	m := make(map[string]string)

	if f.TargetCluster != "" {
		m["target_cluster"] = f.TargetCluster
	}

	return m
}

func (f *ValidatedMFAChallengeFilter) FromMap(m map[string]string) error {
	for key, val := range m {
		switch key {
		case "target_cluster":
			f.TargetCluster = val

		default:
			return trace.BadParameter("unknown filter key %s", key)
		}
	}

	return nil
}

// TODO: Resolve import cycle to use this filter in events.go!!!
func (f *ValidatedMFAChallengeFilter) Match(r mfav1.ValidatedMFAChallenge) bool {
	return r.GetSpec().GetTargetCluster() == f.TargetCluster
}
