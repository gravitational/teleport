package resourceregistry

import (
	"testing"

	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestPrototypeSpecs(t *testing.T) {
	registry := New()
	require.NoError(t, Register(registry, AccessMonitoringRuleSpec()))
	require.NoError(t, Register(registry, RoleSpec()))
	require.ElementsMatch(t, []string{types.KindAccessMonitoringRule, types.KindRole}, registry.Kinds())

	amrSpec, err := Get[*accessmonitoringrulesv1.AccessMonitoringRule, NameID](registry, types.KindAccessMonitoringRule)
	require.NoError(t, err)

	rule, err := services.NewAccessMonitoringRuleWithLabels("amr-example", nil, &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
		Subjects:  []string{types.KindRole},
		Condition: "true",
	})
	require.NoError(t, err)
	rule.GetMetadata().SetRevision("amr-rev")

	require.Equal(t, NameID("amr-example"), amrSpec.ID(rule))
	require.Equal(t, "amr-rev", amrSpec.Revision(rule))
	require.NotSame(t, rule, amrSpec.Clone(rule))

	rule153, err := ToResource153(rule)
	require.NoError(t, err)
	ruleAgain, err := FromResource153[*accessmonitoringrulesv1.AccessMonitoringRule](rule153)
	require.NoError(t, err)
	require.Equal(t, "amr-example", ruleAgain.GetMetadata().GetName())

	roleSpec, err := Get[types.Role, NameID](registry, types.KindRole)
	require.NoError(t, err)

	role, err := types.NewRole("role-example", types.RoleSpecV6{})
	require.NoError(t, err)
	role.SetRevision("role-rev")

	require.Equal(t, NameID("role-example"), roleSpec.ID(role))
	require.Equal(t, "role-rev", roleSpec.Revision(role))
	require.NotSame(t, role, roleSpec.Clone(role))

	role153, err := ToResource153(role)
	require.NoError(t, err)
	roleAgain, err := FromResource153[types.Role](role153)
	require.NoError(t, err)
	require.Equal(t, "role-example", roleAgain.GetName())
}
