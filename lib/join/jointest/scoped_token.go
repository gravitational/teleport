// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package jointest

import (
	"cmp"
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

const (
	testTokenScope         = "/test"
	testTokenAssignedScope = "/test/one"
)

var testTokenImmutableLabels = map[string]string{"scoped-join-test": "true"}

// ScopedTokenClient is the subset of the scoped token service used by join
// integration tests.
type ScopedTokenClient interface {
	CreateScopedToken(context.Context, *joiningv1.CreateScopedTokenRequest) (*joiningv1.CreateScopedTokenResponse, error)
	DeleteScopedToken(context.Context, *joiningv1.DeleteScopedTokenRequest) (*joiningv1.DeleteScopedTokenResponse, error)
}

// CreateScopedToken creates a dynamic scoped token equivalent to base and
// registers cleanup. Provider configuration is intentionally derived from the
// classic token so the same high-level test matrix can exercise both forms.
func CreateScopedToken(
	t testing.TB,
	client ScopedTokenClient,
	base types.ProvisionTokenSpecV2,
	name string,
) *joiningv1.ScopedToken {
	t.Helper()

	token, err := ScopedTokenFromProvisionTokenSpec(base, joiningv1.ScopedToken_builder{
		Scope: testTokenScope,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: testTokenAssignedScope,
			UsageMode:     string(joining.TokenUsageModeUnlimited),
			ImmutableLabels: joiningv1.ImmutableLabels_builder{
				Ssh: testTokenImmutableLabels,
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	_, err = client.CreateScopedToken(t.Context(), joiningv1.CreateScopedTokenRequest_builder{
		Token: token,
	}.Build())
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := client.DeleteScopedToken(context.Background(), joiningv1.DeleteScopedTokenRequest_builder{
			Name: token.GetMetadata().GetName(),
		}.Build())
		require.NoError(t, err)
	})

	return token
}

// ScopedTokenFromProvisionTokenSpec is a test helper that creates a scoped token using a [types.ProvisionTokenSpecV2]
// as a base. The override parameter can be used to provide override values that should be used instead of the base token
// values. This is mostly useful for testing.
func ScopedTokenFromProvisionTokenSpec(base types.ProvisionTokenSpecV2, override *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	roles := types.SystemRoles(base.Roles).StringSlice()
	if len(roles) == 0 {
		roles = override.GetSpec().GetRoles()
	}

	// scoped tokens can't be created without a join method, so we replicate ProvisionTokenV2.CheckAndSetDefaults() here
	// for testing purposes
	if base.JoinMethod == types.JoinMethodUnspecified {
		base.JoinMethod = types.JoinMethodToken
		if len(base.Allow) > 0 {
			base.JoinMethod = types.JoinMethodEC2
		}
	}

	scopedToken := joiningv1.ScopedToken_builder{
		Kind:     types.KindScopedToken,
		Version:  types.V1,
		Scope:    override.GetScope(),
		Metadata: override.GetMetadata(),
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope:   override.GetSpec().GetAssignedScope(),
			JoinMethod:      cmp.Or(override.GetSpec().GetJoinMethod(), string(base.JoinMethod)),
			Roles:           roles,
			UsageMode:       override.GetSpec().GetUsageMode(),
			ImmutableLabels: override.GetSpec().GetImmutableLabels(),
		}.Build(),
	}.Build()

	switch base.JoinMethod {
	case types.JoinMethodEC2:
		allow := make([]*joiningv1.AWS_Rule, len(base.Allow))
		for i, rule := range base.Allow {
			allow[i] = joiningv1.AWS_Rule_builder{
				AwsAccount:        rule.AWSAccount,
				AwsRegions:        rule.AWSRegions,
				AwsRole:           rule.AWSRole,
				AwsArn:            rule.AWSARN,
				AwsOrganizationId: rule.AWSOrganizationID,
			}.Build()
		}
		scopedToken.GetSpec().SetAws(joiningv1.AWS_builder{
			Allow:  allow,
			IidTtl: base.AWSIIDTTL.Duration().String(),
		}.Build())
	case types.JoinMethodIAM:
		allow := make([]*joiningv1.AWS_Rule, len(base.Allow))
		for i, rule := range base.Allow {
			allow[i] = joiningv1.AWS_Rule_builder{
				AwsAccount:        rule.AWSAccount,
				AwsRegions:        rule.AWSRegions,
				AwsRole:           rule.AWSRole,
				AwsArn:            rule.AWSARN,
				AwsOrganizationId: rule.AWSOrganizationID,
			}.Build()
		}
		scopedToken.GetSpec().SetAws(joiningv1.AWS_builder{
			Allow:       allow,
			Integration: base.Integration,
		}.Build())
	case types.JoinMethodGCP:
		allow := make([]*joiningv1.GCP_Rule, len(base.GCP.Allow))
		for i, rule := range base.GCP.Allow {
			allow[i] = joiningv1.GCP_Rule_builder{
				ProjectIds:      rule.ProjectIDs,
				Locations:       rule.Locations,
				ServiceAccounts: rule.ServiceAccounts,
			}.Build()
		}
		scopedToken.GetSpec().SetGcp(joiningv1.GCP_builder{
			Allow: allow,
		}.Build())
	case types.JoinMethodAzure:
		allow := make([]*joiningv1.Azure_Rule, len(base.Azure.Allow))
		for i, rule := range base.Azure.Allow {
			allow[i] = joiningv1.Azure_Rule_builder{
				Tenant:         rule.Tenant,
				Subscription:   rule.Subscription,
				ResourceGroups: rule.ResourceGroups,
			}.Build()
		}
		scopedToken.GetSpec().SetAzure(joiningv1.Azure_builder{
			Allow: allow,
		}.Build())
	case types.JoinMethodAzureDevops:
		allow := make([]*joiningv1.AzureDevops_Rule, len(base.AzureDevops.Allow))
		for i, rule := range base.AzureDevops.Allow {
			allow[i] = joiningv1.AzureDevops_Rule_builder{
				Sub:               rule.Sub,
				ProjectName:       rule.ProjectName,
				PipelineName:      rule.PipelineName,
				ProjectId:         rule.ProjectID,
				DefinitionId:      rule.DefinitionID,
				RepositoryUri:     rule.RepositoryURI,
				RepositoryVersion: rule.RepositoryVersion,
				RepositoryRef:     rule.RepositoryRef,
			}.Build()
		}
		scopedToken.GetSpec().SetAzureDevops(joiningv1.AzureDevops_builder{
			Allow:          allow,
			OrganizationId: base.AzureDevops.OrganizationID,
		}.Build())
	case types.JoinMethodOracle:
		allow := make([]*joiningv1.Oracle_Rule, len(base.Oracle.Allow))
		for i, rule := range base.Oracle.Allow {
			allow[i] = joiningv1.Oracle_Rule_builder{
				Tenancy:            rule.Tenancy,
				ParentCompartments: rule.ParentCompartments,
				Regions:            rule.Regions,
				Instances:          rule.Instances,
			}.Build()
		}
		scopedToken.GetSpec().SetOracle(joiningv1.Oracle_builder{
			Allow: allow,
		}.Build())
	case types.JoinMethodKubernetes:
		if base.Kubernetes == nil {
			return nil, trace.BadParameter("kubernetes configuration must be defined for kubernetes join method")
		}
		allow := make([]*joiningv1.Kubernetes_Rule, len(base.Kubernetes.Allow))
		for i, rule := range base.Kubernetes.Allow {
			allow[i] = joiningv1.Kubernetes_Rule_builder{
				ServiceAccount:          rule.ServiceAccount,
				ServiceAccountNamespace: rule.ServiceAccountNamespace,
				ServiceAccountName:      rule.ServiceAccountName,
			}.Build()
		}

		var staticJWKS *joiningv1.Kubernetes_StaticJWKSConfig
		if base.Kubernetes.StaticJWKS != nil {
			staticJWKS = joiningv1.Kubernetes_StaticJWKSConfig_builder{
				Jwks: base.Kubernetes.StaticJWKS.JWKS,
			}.Build()
		}

		var oidc *joiningv1.Kubernetes_OIDCConfig
		if base.Kubernetes.OIDC != nil {
			oidc = joiningv1.Kubernetes_OIDCConfig_builder{
				Issuer:                  base.Kubernetes.OIDC.Issuer,
				InsecureAllowHttpIssuer: base.Kubernetes.OIDC.InsecureAllowHTTPIssuer,
			}.Build()
		}

		scopedToken.GetSpec().SetKubernetes(joiningv1.Kubernetes_builder{
			Allow:      allow,
			Type:       string(base.Kubernetes.Type),
			StaticJwks: staticJWKS,
			Oidc:       oidc,
		}.Build())
	case types.JoinMethodGitHub:
		if err := setProviderConfig(scopedToken.GetSpec(), "github", base.GitHub); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported join method %q", base.JoinMethod)
	}

	return scopedToken, nil
}

// setProviderConfig populates a provider message by protobuf field name. This
// keeps forward-looking integration tests buildable before the generated scoped
// provider type exists, while still failing until the production schema defines
// and can decode the expected field.
func setProviderConfig(spec *joiningv1.ScopedTokenSpec, fieldName string, config any) error {
	if config == nil {
		return trace.BadParameter("missing %s configuration", fieldName)
	}

	message := spec.ProtoReflect()
	field := message.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field == nil {
		return trace.NotImplemented("scoped token proto does not define %q configuration", fieldName)
	}
	if field.Kind() != protoreflect.MessageKind {
		return trace.BadParameter("scoped token field %q must be a message", fieldName)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		return trace.Wrap(err, "marshaling classic %s configuration", fieldName)
	}

	value := message.NewField(field)
	if err := protojson.Unmarshal(encoded, value.Message().Interface()); err != nil {
		return trace.Wrap(err, "converting classic %s configuration to scoped form", fieldName)
	}
	message.Set(field, value)

	return nil
}
