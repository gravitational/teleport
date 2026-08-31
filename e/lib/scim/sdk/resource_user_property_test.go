package scimsdk

import (
	"encoding/json"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/utils/set"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func genNonEmptyString() *rapid.Generator[string] {
	return rapid.StringN(1, -1, -1)
}

func genName() *rapid.Generator[Name] {
	return rapid.Custom(func(t *rapid.T) Name {
		return Name{
			FamilyName: rapid.String().Draw(t, "familyName"),
			GivenName:  rapid.String().Draw(t, "givenName"),
		}
	})
}

func nameAsJSON(n *Name) map[string]any {
	if n == nil {
		return nil
	}
	attrs := map[string]any{}
	if n.GivenName != "" {
		attrs["givenName"] = n.GivenName
	}
	if n.FamilyName != "" {
		attrs["familyName"] = n.FamilyName
	}
	if len(attrs) == 0 {
		return nil
	}
	return map[string]any{"name": attrs}
}

func genTime() *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		return time.UnixMilli(rapid.Int64Range(0, 2000000000000).Draw(t, "unixMillis")).UTC()
	})
}

func genMetadata() *rapid.Generator[Metadata] {
	return rapid.Custom(func(t *rapid.T) Metadata {
		resourceType := genNonEmptyString().Draw(t, "resourceType")
		created := rapid.Ptr(genTime(), true /* allowNil */).Draw(t, "created")
		lastModified := rapid.Ptr(genTime(), true /* allowNil */).Draw(t, "lastModified")
		location := rapid.String().Draw(t, "location")
		version := rapid.String().Draw(t, "version")

		return Metadata{
			ResourceType: resourceType,
			Created:      created,
			LastModified: lastModified,
			Location:     location,
			Version:      version,
		}
	})
}

func metadataAsJSON(m *Metadata) map[string]any {
	if m == nil {
		return nil
	}

	attrs := map[string]any{}
	if m.ResourceType != "" {
		attrs["resourceType"] = m.ResourceType
	}

	if m.Created != nil {
		attrs["created"] = m.Created.Format(time.RFC3339Nano)
	}

	if m.LastModified != nil {
		attrs["lastModified"] = m.LastModified.Format(time.RFC3339Nano)
	}

	if m.Location != "" {
		attrs["location"] = m.Location
	}

	if m.Version != "" {
		attrs["version"] = m.Version
	}

	if len(attrs) == 0 {
		return nil
	}

	return map[string]any{"meta": attrs}
}

func asAny[T any](v T) any {
	return v
}

func isNotEmpty[T comparable](v T) bool {
	var empty T
	return v != empty
}

func genNil() *rapid.Generator[any] {
	return rapid.Custom(func(t *rapid.T) any {
		// Check will fail if we don't consume any randomness, so read and discard
		// an arbitrary value to avoid the error
		rapid.Int().Draw(t, "ignored")
		return nil
	})
}

func genJSONValue(depth int) *rapid.Generator[any] {
	jsonValue := rapid.Deferred(func() *rapid.Generator[any] {
		if depth == 0 {
			return rapid.Float64().AsAny()
		}
		return genJSONValue(depth - 1)
	})
	return rapid.OneOf(
		rapid.String().AsAny(),
		rapid.Bool().AsAny(),
		rapid.Float64().AsAny(),
		genNil(),
		rapid.MapOf(genNonEmptyString(), jsonValue).AsAny(),
		rapid.SliceOf(jsonValue).AsAny())
}

func TestUser_Property(t *testing.T) {
	reservedAttributes := set.New(knownStructFields...)
	isNotReserved := func(s string) bool {
		return !reservedAttributes.Contains(s)
	}

	rapid.Check(t, func(t *rapid.T) {
		userID := genNonEmptyString().Draw(t, "userID")
		meta := rapid.Ptr(genMetadata(), true /* allowNil */).Draw(t, "Metadata")
		externalID := genNonEmptyString().Draw(t, "externalID")
		userName := genNonEmptyString().Draw(t, "userName")
		schemas := rapid.SliceOfN(genNonEmptyString(), 1, -1).Draw(t, "schemas")
		displayName := genNonEmptyString().Draw(t, "displayName")
		name := rapid.Ptr(genName().Filter(isNotEmpty), true /* allowNil */).Draw(t, "name")
		active := rapid.Bool().Draw(t, "active")
		attributes := rapid.MapOf(
			genNonEmptyString().Filter(isNotReserved),
			genJSONValue( /*maxDepth*/ 3)).Draw(t, "attributes")

		src := User{
			ID:          userID,
			Meta:        meta,
			ExternalID:  externalID,
			Schemas:     schemas,
			UserName:    userName,
			Name:        name,
			DisplayName: displayName,
			Active:      active,
			Attributes:  AttributeSet(attributes),
		}

		text, err := json.Marshal(&src)
		require.NoError(t, err, "User must serialize cleanly")

		var actualJSON map[string]any
		err = json.Unmarshal(text, &actualJSON)
		require.NoError(t, err, "User must deserialize into JSON attribute map")

		expectedJSON := map[string]any{
			"id":          userID,
			"externalId":  externalID,
			"schemas":     sliceutils.Map(schemas, asAny),
			"userName":    userName,
			"displayName": displayName,
			// the active field must always be present in the serialized JSON, even when false.
			"active": active,
		}
		maps.Copy(expectedJSON, metadataAsJSON(meta))
		maps.Copy(expectedJSON, nameAsJSON(name))
		maps.Copy(expectedJSON, attributes)
		require.Equal(t, expectedJSON, actualJSON, "User must serialize to expected JSON")

		var dst User
		err = json.Unmarshal(text, &dst)
		require.NoError(t, err, "User must deserialize cleanly")
		require.Equal(t, src, dst, "User must round-trip through JSON")
	})
}
