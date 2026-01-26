package conv

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func Test_setSCIMAttrsInUserLabel(t *testing.T) {
	attrs, err := structpb.NewStruct(map[string]any{
		"active":   true,
		"emails":   map[string]any{"primary": true, "type": "work", "value": "alice@email.test"},
		"name":     map[string]any{"givenName": "Alice", "familyName": "Okta"},
		"userName": "alice@example.com",
		"extra":    "stuff",
		"password": "pa$$word", // should be omitted
	})
	require.NoError(t, err)
	r := &scimpb.Resource{
		Attributes: attrs,
	}

	const expectedJSON = `{"active":true,"emails":{"primary":true,"type":"work","value":"alice@email.test"},"name":{"givenName":"Alice","familyName":"Okta"},"userName":"alice@example.com","extra":"stuff"}`

	// Happy path - enough space.
	u, err := types.NewUser("alice")
	require.NoError(t, err)

	err = setSCIMAttrsInUserLabel(u, r, len(expectedJSON))
	require.NoError(t, err)
	v, ok := u.GetLabel(eteleport.SCIMAttrsLabel)
	require.True(t, ok)
	requireEqualJSON(t, expectedJSON, v)

	// Too large.
	u, err = types.NewUser("alice")
	require.NoError(t, err)

	err = setSCIMAttrsInUserLabel(u, r, len(expectedJSON)-1)
	require.ErrorContains(t, err, "teleport.internal/scim-attrs label value too large")
	require.True(t, trace.IsBadParameter(err))
}

func requireEqualJSON(t *testing.T, expectedJSON, actualJSON string) {
	t.Helper()
	var expected, actual any
	err := json.Unmarshal([]byte(expectedJSON), &expected)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(actualJSON), &actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
