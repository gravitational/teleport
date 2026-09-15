package patch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractMemberOps(t *testing.T) {
	t.Parallel()

	checkOps := func(want []MemberOp) func(t *testing.T, got []MemberOp, ok bool) {
		return func(t *testing.T, got []MemberOp, ok bool) {
			t.Helper()
			require.True(t, ok)
			require.Equal(t, want, got)
		}
	}

	checkFullPatchPicked := func(t *testing.T, got []MemberOp, ok bool) {
		t.Helper()
		require.False(t, ok)
	}

	tests := []struct {
		name  string
		patch string
		check func(t *testing.T, got []MemberOp, ok bool)
	}{
		{
			name: "add single member",
			patch: `{
				"Operations": [
					{ "op": "add", "path": "members", "value": [ { "value": "alice" } ] }
				]
			}`,
			check: checkOps([]MemberOp{{Action: MemberAdd, Value: "alice"}}),
		},
		{
			name: "add multiple members",
			patch: `{
				"Operations": [
					{ "op": "add", "path": "members", "value": [ { "value": "alice" }, { "value": "bob" } ] }
				]
			}`,
			check: checkOps([]MemberOp{{Action: MemberAdd, Value: "alice"}, {Action: MemberAdd, Value: "bob"}}),
		},
		{
			name: "remove single member by filter",
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "members[value eq \"alice\"]" }
				]
			}`,
			check: checkOps([]MemberOp{{Action: MemberRemove, Value: "alice"}}),
		},
		{
			name: "mixed add and remove",
			patch: `{
				"Operations": [
					{ "op": "add", "path": "members", "value": [ { "value": "alice" } ] },
					{ "op": "remove", "path": "members[value eq \"bob\"]" }
				]
			}`,
			check: checkOps([]MemberOp{{Action: MemberAdd, Value: "alice"}, {Action: MemberRemove, Value: "bob"}}),
		},
		{
			name: "non-members attribute falls back",
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "displayName", "value": "new-name" }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name: "mixed members and non-members falls back",
			patch: `{
				"Operations": [
					{ "op": "add", "path": "members", "value": [ { "value": "alice" } ] },
					{ "op": "replace", "path": "displayName", "value": "new-name" }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name: "bulk replace of members falls back",
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "members", "value": [ { "value": "alice" } ] }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name: "remove all members with no filter falls back",
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "members" }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name: "remove with compound filter falls back",
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "members[value eq \"alice\" or value eq \"bob\"]" }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name: "add with non-array value falls back",
			patch: `{
				"Operations": [
					{ "op": "add", "path": "members", "value": "alice" }
				]
			}`,
			check: checkFullPatchPicked,
		},
		{
			name:  "no operations returns empty",
			patch: `{ "Operations": [] }`,
			check: checkOps(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractMemberOps([]byte(tt.patch))
			tt.check(t, got, ok)
		})
	}
}
