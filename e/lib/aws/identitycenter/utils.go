package identitycenter

import (
	"regexp"
	"strings"
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
