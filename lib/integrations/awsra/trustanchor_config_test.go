/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package awsra

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/aws/tags"
)

var badParameterCheck = func(t require.TestingT, err error, msgAndArgs ...any) {
	require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
}

func TestConfigureRolesAnywhereIAMReqDefaults(t *testing.T) {
	baseRolesAnywhereConfigReq := func() TrustAnchorConfigureRequest {
		return TrustAnchorConfigureRequest{
			Cluster:               "mycluster",
			AccountID:             "123456789012",
			IntegrationName:       "myintegration",
			TrustAnchorName:       "mytrustanchor",
			TrustAnchorCertBase64: base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIID...==\n-----END CERTIFICATE-----")),
			SyncProfileName:       "mysyncprofile",
			SyncRoleName:          "myrole",
			AutoConfirm:           true,
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() TrustAnchorConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected TrustAnchorConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseRolesAnywhereConfigReq,
			errCheck: require.NoError,
			expected: TrustAnchorConfigureRequest{
				Cluster:               "mycluster",
				AccountID:             "123456789012",
				IntegrationName:       "myintegration",
				TrustAnchorName:       "mytrustanchor",
				TrustAnchorCertBase64: base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIID...==\n-----END CERTIFICATE-----")),
				SyncProfileName:       "mysyncprofile",
				SyncRoleName:          "myrole",
				AutoConfirm:           true,
				ownershipTags: tags.AWSTags{
					"teleport.dev/origin":      "integration_awsrolesanywhere",
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
				},
				stdout: os.Stdout,
			},
		},
		{
			name: "missing cluster name",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.Cluster = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration name",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.IntegrationName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing trust anchor name",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.TrustAnchorName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing trust anchor cert",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.TrustAnchorCertBase64 = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing sync profile name",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.SyncProfileName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing sync role name",
			req: func() TrustAnchorConfigureRequest {
				req := baseRolesAnywhereConfigReq()
				req.SyncRoleName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req()
			err := req.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, req)
		})
	}
}

func TestConfigureRolesAnywhereTrustAnchor(t *testing.T) {
	ctx := context.Background()

	baseRolesAnywhereConfigReq := func() TrustAnchorConfigureRequest {
		return TrustAnchorConfigureRequest{
			Cluster:               "mycluster",
			AccountID:             "123456789012",
			IntegrationName:       "myintegration",
			TrustAnchorName:       "mytrustanchor",
			TrustAnchorCertBase64: base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIID...==\n-----END CERTIFICATE-----")),
			SyncProfileName:       "mysyncprofile",
			SyncRoleName:          "mysyncrole",
			AutoConfirm:           true,
		}
	}

	for _, tt := range []struct {
		name                   string
		req                    func() TrustAnchorConfigureRequest
		existingTrustAnchors   []ratypes.TrustAnchorDetail
		existingProfiles       []ratypes.ProfileDetail
		existingRAResourceTags map[string][]ratypes.Tag
		existingRoles          []mockRole
		trustAnchorID          string
		errCheck               require.ErrorAssertionFunc
		externalStateCheck     func(*testing.T, mockIAMRolesAnywhereClient)
	}{
		{
			name:          "valid",
			req:           baseRolesAnywhereConfigReq,
			errCheck:      require.NoError,
			trustAnchorID: "my-trust-anchor-uuid",
			externalStateCheck: func(t *testing.T, clt mockIAMRolesAnywhereClient) {
				trustAnchors, err := clt.ListTrustAnchors(ctx, &rolesanywhere.ListTrustAnchorsInput{})
				require.NoError(t, err)
				require.Len(t, trustAnchors.TrustAnchors, 1)
				require.Equal(t, "mytrustanchor", aws.ToString(trustAnchors.TrustAnchors[0].Name))
				require.Equal(t, "my-trust-anchor-uuid", aws.ToString(trustAnchors.TrustAnchors[0].TrustAnchorId))

				syncRoleTrustPolicy := clt.mockIAMRoles.existingRoles["mysyncrole"].assumeRolePolicyDoc
				expectedSyncRoleTrustPolicy := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sts:AssumeRole",
                "sts:SetSourceIdentity",
                "sts:TagSession"
            ],
            "Principal": {
                "Service": "rolesanywhere.amazonaws.com"
            },
            "Condition": {
                "ArnEquals": {
                    "aws:SourceArn": "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/my-trust-anchor-uuid"
                }
            }
        }
    ]
}`
				require.Equal(t, expectedSyncRoleTrustPolicy, aws.ToString(syncRoleTrustPolicy))
			},
		},
		{
			name: "trust anchor already exists but is missing the required ownership tags",
			req:  baseRolesAnywhereConfigReq,
			existingTrustAnchors: []ratypes.TrustAnchorDetail{{
				Name: aws.String("mytrustanchor"),
			}},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "is not owned by this integration")
			},
		},
		{
			name: "profile already exists but is missing the required ownership tags",
			req:  baseRolesAnywhereConfigReq,
			existingProfiles: []ratypes.ProfileDetail{{
				Name: aws.String("mysyncprofile"),
			}},
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "is not owned by this integration")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			existingTrustAnchors := make(map[string]ratypes.TrustAnchorDetail, len(tt.existingTrustAnchors))
			for _, trustAnchor := range tt.existingTrustAnchors {
				existingTrustAnchors[aws.ToString(trustAnchor.TrustAnchorId)] = trustAnchor
			}

			existingProfiles := make(map[string]ratypes.ProfileDetail, len(tt.existingProfiles))
			for _, profile := range tt.existingProfiles {
				existingProfiles[aws.ToString(profile.ProfileId)] = profile
			}

			existingResourceTags := make(map[string][]ratypes.Tag, len(tt.existingRAResourceTags))
			maps.Copy(existingResourceTags, tt.existingRAResourceTags)

			existingRoles := make(map[string]iamtypes.Role, len(tt.existingRoles))
			for _, role := range tt.existingRoles {
				existingRoles[role.name] = iamtypes.Role{
					Arn:                      aws.String("arn:aws:iam::123456789012:role/" + role.name),
					AssumeRolePolicyDocument: role.assumeRolePolicyDoc,
					Tags:                     role.tags,
				}
			}

			clt := mockIAMRolesAnywhereClient{
				mockIAMRolesAnywhere: mockIAMRolesAnywhere{
					trustAnchorsByID: existingTrustAnchors,
					profilesByID:     existingProfiles,
					resourceTags:     existingResourceTags,
					trustAnchorID:    tt.trustAnchorID,
				},
				mockIAMRoles: mockIAMRoles{
					existingRoles: make(map[string]mockRole),
				},
			}

			err := ConfigureRolesAnywhereIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)

			if tt.externalStateCheck != nil {
				tt.externalStateCheck(t, clt)
			}
		})
	}
}

type mockIAMRolesAnywhere struct {
	trustAnchorID    string
	trustAnchorsByID map[string]ratypes.TrustAnchorDetail
	profilesByID     map[string]ratypes.ProfileDetail
	resourceTags     map[string][]ratypes.Tag
}

func (m *mockIAMRolesAnywhere) ListTrustAnchors(ctx context.Context, params *rolesanywhere.ListTrustAnchorsInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTrustAnchorsOutput, error) {
	return &rolesanywhere.ListTrustAnchorsOutput{
		TrustAnchors: slices.Collect(maps.Values(m.trustAnchorsByID)),
	}, nil
}

func (m *mockIAMRolesAnywhere) CreateTrustAnchor(ctx context.Context, params *rolesanywhere.CreateTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.CreateTrustAnchorOutput, error) {
	newTrustAnchorID := uuid.NewString()
	if m.trustAnchorID != "" {
		newTrustAnchorID = m.trustAnchorID
	}
	newTrustAnchorARN := "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/" + newTrustAnchorID

	m.trustAnchorsByID[newTrustAnchorID] = ratypes.TrustAnchorDetail{
		Name:           params.Name,
		TrustAnchorArn: aws.String(newTrustAnchorARN),
		TrustAnchorId:  aws.String(newTrustAnchorID),
		Enabled:        params.Enabled,
	}

	m.resourceTags[newTrustAnchorARN] = params.Tags

	return nil, nil
}

func (m *mockIAMRolesAnywhere) UpdateTrustAnchor(ctx context.Context, params *rolesanywhere.UpdateTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.UpdateTrustAnchorOutput, error) {
	existingTrustAnchor, ok := m.trustAnchorsByID[aws.ToString(params.TrustAnchorId)]
	if !ok {
		return nil, trace.NotFound("trust anchor %q not found", aws.ToString(params.TrustAnchorId))
	}
	existingTrustAnchor.Source = params.Source

	m.trustAnchorsByID[aws.ToString(params.TrustAnchorId)] = existingTrustAnchor
	return nil, nil
}

func (m *mockIAMRolesAnywhere) EnableTrustAnchor(ctx context.Context, params *rolesanywhere.EnableTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.EnableTrustAnchorOutput, error) {
	existingTrustAnchor, ok := m.trustAnchorsByID[aws.ToString(params.TrustAnchorId)]
	if !ok {
		return nil, trace.NotFound("trust anchor %q not found", aws.ToString(params.TrustAnchorId))
	}
	existingTrustAnchor.Enabled = aws.Bool(true)

	m.trustAnchorsByID[aws.ToString(params.TrustAnchorId)] = existingTrustAnchor
	return nil, nil
}

func (m *mockIAMRolesAnywhere) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: m.resourceTags[aws.ToString(params.ResourceArn)],
	}, nil
}

func (m *mockIAMRolesAnywhere) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	if m.profilesByID == nil {
		m.profilesByID = make(map[string]ratypes.ProfileDetail)
	}

	allProfiles := slices.Collect(maps.Values(m.profilesByID))
	slices.SortFunc(allProfiles, func(a, b ratypes.ProfileDetail) int {
		return strings.Compare(aws.ToString(a.ProfileArn), aws.ToString(b.ProfileArn))
	})

	return &rolesanywhere.ListProfilesOutput{
		Profiles: allProfiles,
	}, nil
}

func (m *mockIAMRolesAnywhere) CreateProfile(ctx context.Context, params *rolesanywhere.CreateProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.CreateProfileOutput, error) {
	newProfileID := uuid.NewString()
	newProfileARN := "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/" + newProfileID

	m.profilesByID[newProfileID] = ratypes.ProfileDetail{
		Name:       params.Name,
		ProfileArn: aws.String(newProfileARN),
		ProfileId:  aws.String(newProfileID),
		Enabled:    params.Enabled,
	}

	m.resourceTags[newProfileARN] = params.Tags

	return nil, nil
}

func (m *mockIAMRolesAnywhere) UpdateProfile(ctx context.Context, params *rolesanywhere.UpdateProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.UpdateProfileOutput, error) {
	existingProfile, ok := m.profilesByID[aws.ToString(params.ProfileId)]
	if !ok {
		return nil, trace.NotFound("trust anchor %q not found", aws.ToString(params.ProfileId))
	}
	existingProfile.AcceptRoleSessionName = params.AcceptRoleSessionName
	existingProfile.RoleArns = params.RoleArns

	m.profilesByID[aws.ToString(params.ProfileId)] = existingProfile
	return nil, nil
}

func (m *mockIAMRolesAnywhere) EnableProfile(ctx context.Context, params *rolesanywhere.EnableProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.EnableProfileOutput, error) {
	existingProfile, ok := m.profilesByID[aws.ToString(params.ProfileId)]
	if !ok {
		return nil, trace.NotFound("trust anchor %q not found", aws.ToString(params.ProfileId))
	}
	existingProfile.Enabled = aws.Bool(true)

	m.profilesByID[aws.ToString(params.ProfileId)] = existingProfile
	return nil, nil
}

type mockIAMRolesAnywhereClient struct {
	mockIAMRolesAnywhere
	mockIAMRoles
}

func (m *mockIAMRolesAnywhereClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String("123456789012"),
	}, nil
}

type mockRole struct {
	name                string
	assumeRolePolicyDoc *string
	tags                []iamtypes.Tag
	presetPolicyDoc     *string
}

type mockIAMRoles struct {
	existingRoles map[string]mockRole
}

// CreateRole creates a new IAM Role.
func (m *mockIAMRoles) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("Role %q already exists.", *params.RoleName)
	_, found := m.existingRoles[aws.ToString(params.RoleName)]
	if found {
		return nil, &iamtypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles[*params.RoleName] = mockRole{
		tags:                params.Tags,
		assumeRolePolicyDoc: params.AssumeRolePolicyDocument,
	}

	return &iam.CreateRoleOutput{
		Role: &iamtypes.Role{
			Arn: aws.String("arn:something"),
		},
	}, nil
}

// PutRolePolicy assigns a policy to an existing IAM Role.
func (m *mockIAMRoles) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	doesNotExistMessage := fmt.Sprintf("Role %q does not exist.", *params.RoleName)
	if _, ok := m.existingRoles[aws.ToString(params.RoleName)]; !ok {
		return nil, &iamtypes.NoSuchEntityException{
			Message: &doesNotExistMessage,
		}
	}

	m.existingRoles[*params.RoleName] = mockRole{
		tags:                m.existingRoles[*params.RoleName].tags,
		assumeRolePolicyDoc: m.existingRoles[*params.RoleName].assumeRolePolicyDoc,
		presetPolicyDoc:     params.PolicyDocument,
	}

	return &iam.PutRolePolicyOutput{}, nil
}

// GetRole retrieves information about the specified role, including the role's path,
// GUID, ARN, and the role's trust policy that grants permission to assume the
// role.
func (m *mockIAMRoles) GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	role, found := m.existingRoles[aws.ToString(params.RoleName)]
	if !found {
		return nil, trace.NotFound("role not found")
	}
	return &iam.GetRoleOutput{
		Role: &iamtypes.Role{
			Tags:                     role.tags,
			AssumeRolePolicyDocument: role.assumeRolePolicyDoc,
			Arn:                      aws.String("arn:aws:iam::123456789012:role/" + aws.ToString(params.RoleName)),
		},
	}, nil
}

// UpdateAssumeRolePolicy updates the policy that grants an IAM entity permission to assume a role.
// This is typically referred to as the "role trust policy".
func (m *mockIAMRoles) UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error) {
	role, found := m.existingRoles[aws.ToString(params.RoleName)]
	if !found {
		return nil, trace.NotFound("role not found")
	}

	role.assumeRolePolicyDoc = params.PolicyDocument
	m.existingRoles[aws.ToString(params.RoleName)] = role

	return &iam.UpdateAssumeRolePolicyOutput{}, nil
}

func (m *mockIAMRoles) TagRole(ctx context.Context, params *iam.TagRoleInput, _ ...func(*iam.Options)) (*iam.TagRoleOutput, error) {
	roleName := aws.ToString(params.RoleName)
	role, found := m.existingRoles[roleName]
	if !found {
		return nil, trace.NotFound("role not found")
	}

	tags := tags.AWSTags{}
	for _, existingTag := range role.tags {
		tags[*existingTag.Key] = *existingTag.Value
	}
	for _, newTag := range params.Tags {
		tags[*newTag.Key] = *newTag.Value
	}
	role.tags = tags.ToIAMTags()
	m.existingRoles[roleName] = role
	return &iam.TagRoleOutput{}, nil
}
