package jointest

import (
	"cmp"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
)

// ScopedTokenFromProvisionToken is a test helper that creates a scoped token using a [provision.Token]
// as a base. The override parameter can be used to provide override values that should
// be used instead of the base token values. This is mostly useful
// for testing.
func ScopedTokenFromProvisionToken(base provision.Token, override *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	roles := base.GetRoles().StringSlice()
	if len(roles) == 0 {
		roles = override.GetSpec().GetRoles()
	}

	scopedToken := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Scope:   override.GetScope(),
		Metadata: cmp.Or(override.GetMetadata(), &headerv1.Metadata{
			Name: base.GetName(),
		}),
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: override.GetSpec().GetAssignedScope(),
			JoinMethod:    cmp.Or(override.GetSpec().GetJoinMethod(), string(base.GetJoinMethod())),
			Roles:         roles,
			UsageMode:     override.GetSpec().GetUsageMode(),
		},
	}

	switch base.GetJoinMethod() {
	case types.JoinMethodEC2:
		allow := make([]*joiningv1.AWS_Rule, len(base.GetAllowRules()))
		for i, rule := range base.GetAllowRules() {
			allow[i] = &joiningv1.AWS_Rule{
				AwsAccount:        rule.AWSAccount,
				AwsRegions:        rule.AWSRegions,
				AwsRole:           rule.AWSRole,
				AwsArn:            rule.AWSARN,
				AwsOrganizationId: rule.AWSOrganizationID,
			}
		}
		scopedToken.Spec.Aws = &joiningv1.AWS{
			Allow:     allow,
			AwsIidTtl: int64(base.GetAWSIIDTTL()),
		}
	case types.JoinMethodIAM:
		allow := make([]*joiningv1.AWS_Rule, len(base.GetAllowRules()))
		for i, rule := range base.GetAllowRules() {
			allow[i] = &joiningv1.AWS_Rule{
				AwsAccount:        rule.AWSAccount,
				AwsRegions:        rule.AWSRegions,
				AwsRole:           rule.AWSRole,
				AwsArn:            rule.AWSARN,
				AwsOrganizationId: rule.AWSOrganizationID,
			}
		}
		scopedToken.Spec.Aws = &joiningv1.AWS{
			Allow:       allow,
			Integration: base.GetIntegration(),
		}
	case types.JoinMethodGCP:
		allow := make([]*joiningv1.GCP_Rule, len(base.GetGCPRules().Allow))
		for i, rule := range base.GetGCPRules().Allow {
			allow[i] = &joiningv1.GCP_Rule{
				ProjectIds:      rule.ProjectIDs,
				Locations:       rule.Locations,
				ServiceAccounts: rule.ServiceAccounts,
			}
		}
		scopedToken.Spec.Gcp = &joiningv1.GCP{
			Allow: allow,
		}
	default:
		return nil, trace.BadParameter("unsupported join method %q", base.GetJoinMethod())
	}

	return scopedToken, nil
}
