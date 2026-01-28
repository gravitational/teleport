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
		allow := make([]*joiningv1.GCP_Rule, len(base.GetGCP().Allow))
		for i, rule := range base.GetGCP().Allow {
			allow[i] = &joiningv1.GCP_Rule{
				ProjectIds:      rule.ProjectIDs,
				Locations:       rule.Locations,
				ServiceAccounts: rule.ServiceAccounts,
			}
		}
		scopedToken.Spec.Gcp = &joiningv1.GCP{
			Allow: allow,
		}
	case types.JoinMethodAzure:
		allow := make([]*joiningv1.Azure_Rule, len(base.GetAzure().Allow))
		for i, rule := range base.GetAzure().Allow {
			allow[i] = &joiningv1.Azure_Rule{
				Subscription:   rule.Subscription,
				ResourceGroups: rule.ResourceGroups,
			}
		}
		scopedToken.Spec.Azure = &joiningv1.Azure{
			Allow: allow,
		}
	case types.JoinMethodAzureDevops:
		allow := make([]*joiningv1.AzureDevops_Rule, len(base.GetAzureDevops().Allow))
		for i, rule := range base.GetAzureDevops().Allow {
			allow[i] = &joiningv1.AzureDevops_Rule{
				Sub:               rule.Sub,
				ProjectName:       rule.ProjectName,
				PipelineName:      rule.PipelineName,
				ProjectId:         rule.ProjectID,
				DefinitionId:      rule.DefinitionID,
				RepositoryUri:     rule.RepositoryURI,
				RepositoryVersion: rule.RepositoryVersion,
				RepositoryRef:     rule.RepositoryRef,
			}
		}
		scopedToken.Spec.AzureDevops = &joiningv1.AzureDevops{
			Allow:          allow,
			OrganizationId: base.GetAzureDevops().OrganizationID,
		}
	default:
		return nil, trace.BadParameter("unsupported join method %q", base.GetJoinMethod())
	}

	return scopedToken, nil
}
