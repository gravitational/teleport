package types_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestDelegationConversion(t *testing.T) {
	orig := delegationv1.Delegation_builder{
		Bot: delegationv1.BotDelegator_builder{
			Name:  "claude",
			Scope: "/dev",
		}.Build(),
		Previous: delegationv1.Delegation_builder{
			User: delegationv1.UserDelegator_builder{
				Username: "alice",
			}.Build(),
		}.Build(),
	}.Build()

	// Convert to legacy proto.
	legacy := types.DelegationToLegacy(orig)

	// Convert back to modern proto.
	modern := types.DelegationFromLegacy(legacy)
	require.Empty(t, cmp.Diff(orig, modern, protocmp.Transform()))
}
