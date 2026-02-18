package identitycenter

import (
	"log/slog"
	"regexp"
	"strings"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

// passThrough generates a function that casts a strongly typed map into its
// underlying map type and returns it. For use with the reconciler data getters.
func passThrough[M ~map[K]V, K comparable, V any](m M) func() map[K]V {
	return func() map[K]V {
		return map[K]V(m)
	}
}

var allowCharacters = regexp.MustCompile(`^[0-9a-z\-@:]+$`)

func normalizeResourceName(name string) string {
	name = strings.ToLower(name)

	var sb strings.Builder
	for _, r := range name {
		if allowCharacters.MatchString(string(r)) {
			sb.WriteRune(r)
			continue
		}
		// Replace disallowed characters with '_'
		if sb.Len() > 0 && sb.String()[sb.Len()-1] != '_' {
			sb.WriteRune('_')
		}
	}
	return strings.Trim(sb.String(), "_")
}

func principalAssignmentAttr(s *identitycenterv1.PrincipalAssignment) slog.Attr {
	return slog.Any("principal_assignment", principalAssignmentValuer{state: s})
}

type principalAssignmentValuer struct {
	state *identitycenterv1.PrincipalAssignment
}

func (psv principalAssignmentValuer) LogValue() slog.Value {
	if psv.state == nil {
		return slog.StringValue("<nil>")
	}

	state := psv.state
	spec := state.GetSpec()
	return slog.GroupValue(
		slog.String("id", state.GetMetadata().GetName()),
		slog.String("principal_id", spec.GetPrincipalId()),
		slog.String("principal_type", spec.GetPrincipalType().String()),
		slog.String("external_id", spec.GetExternalId()))
}
