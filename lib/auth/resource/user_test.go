package resource

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/stretchr/testify/require"
)

func TestTraits(t *testing.T) {
	var tests = []struct {
		traitName string
	}{
		// Windows trait names are URLs.
		{
			traitName: "http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname",
		},
		// Simple strings are the most common trait names.
		{
			traitName: "user-groups",
		},
	}

	for _, tt := range tests {
		user := &UserV2{
			Kind:    KindUser,
			Version: V2,
			Metadata: Metadata{
				Name:      "foo",
				Namespace: defaults.Namespace,
			},
			Spec: UserSpecV2{
				Traits: map[string][]string{
					tt.traitName: {"foo"},
				},
			},
		}

		data, err := json.Marshal(user)
		require.NoError(t, err)

		_, err = UnmarshalUser(data)
		require.NoError(t, err)
	}
}
