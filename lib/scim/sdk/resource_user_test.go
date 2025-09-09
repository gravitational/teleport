package scimsdk

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestUserMarshalUnmarshal(t *testing.T) {
	want := User{
		ID:         "userID",
		ExternalID: "userExternalID",
		Meta: &Metadata{
			ResourceType: "User",
		},
		Schemas:  []string{"userSchema"},
		UserName: "userName",
		Name: &Name{
			FamilyName: "familyName",
			GivenName:  "givenName",
		},
		DisplayName: "displayName",
		Active:      true,
		Attributes:  AttributeSet{"attr1": "value1"},
	}

	buff, err := json.Marshal(&want)
	require.NoError(t, err)

	var attrGot = map[string]any{}
	err = json.Unmarshal(buff, &attrGot)
	require.NoError(t, err)

	attrWant := map[string]any{
		"id":         "userID",
		"externalId": "userExternalID",
		"meta": map[string]any{
			"resourceType": "User",
		},
		"schemas":  []any{"userSchema"},
		"userName": "userName",
		"name": map[string]any{
			"familyName": "familyName",
			"givenName":  "givenName",
		},
		"displayName": "displayName",
		"active":      true,
		"attr1":       "value1",
	}
	require.Empty(t, cmp.Diff(attrWant, attrGot))

	var got User
	require.NoError(t, json.Unmarshal(buff, &got))

	require.Empty(t, cmp.Diff(want, got))
}
