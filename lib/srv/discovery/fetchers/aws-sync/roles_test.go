/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

const rolesTestAccountID = "123456789012"

// errTransientIAM stands in for a retryable IAM failure, e.g. throttling. It is
// deliberately not a NoSuchEntityException: those mean the role is gone, these
// mean we failed to read a role that still exists.
var errTransientIAM = errors.New("Rate exceeded")

// roleFault describes how the fake IAM API misbehaves for a single role.
type roleFault int

const (
	faultNone roleFault = iota
	// faultRoleDeletedInline makes ListRolePolicies report the role as gone,
	// which is what AWS returns when the role is deleted mid-poll.
	faultRoleDeletedInline
	// faultRoleDeletedAfterListPolicies makes ListRolePolicies report the
	// role policies, but GetPolicy AND ListAttachedRolePolicies will report
	// the role as gone.
	faultRoleDeletedAfterListPolicies
	// faultRoleDeletedAttached makes ListAttachedRolePolicies report the role
	// as gone. The role survives the inline fetch and is only detected as
	// deleted by the follow-up call.
	faultRoleDeletedAttached
	// faultPolicyDeleted makes GetRolePolicy report NoSuchEntity. The role
	// itself still exists; only the inline policy went away.
	faultPolicyDeleted
	// faultInlineTransient and faultAttachedTransient are non-NoSuchEntity
	// failures: the role exists but could not be read.
	faultInlineTransient
	faultAttachedTransient
)

// fakeRole is one role in the fake AWS account.
type fakeRole struct {
	inline   map[string]string // policy name -> policy document
	attached []string          // attached policy ARNs
	fault    roleFault
}

// fakeIAMRoles implements the subset of iamClient that pollAWSRoles uses.
type fakeIAMRoles struct {
	// iamClient is embedded so that any unexpected call panics rather than
	// silently returning a zero value.
	iamClient

	// roles contains all the roles that this fake will return on the various
	// API calls to it.
	roles map[string]fakeRole

	// listRolesErr, when set, fails the initial ListRoles call.
	listRolesErr error

	// maxPageSize caps the number of roles returned per ListRoles page. Zero
	// means not to cap it and to use the MaxItems input parameter if provided.
	maxPageSize int
}

// ListRoles implements the AWS ListRoles API over the fake. It supports
// pagination via the MaxSize and Marker input parameters and IsTruncated
// output parameter. Options are ignored. If the fake has listRolesErr
// set, that error is returned instead of the output.
func (f *fakeIAMRoles) ListRoles(_ context.Context, in *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	if f.listRolesErr != nil {
		return nil, f.listRolesErr
	}

	names := slices.Sorted(maps.Keys(f.roles))

	start := 0
	if in.Marker != nil {
		start = slices.Index(names, *in.Marker) + 1
	}

	end := len(names)
	if in.MaxItems != nil || f.maxPageSize > 0 {
		// Cap the number of items returned.
		maxSize := f.maxPageSize
		if in.MaxItems != nil && (maxSize == 0 || int(*in.MaxItems) > maxSize) {
			maxSize = int(*in.MaxItems)
		}
		end = min(end, start+maxSize)
	}

	out := &iam.ListRolesOutput{}
	for _, name := range names[start:end] {
		out.Roles = append(out.Roles, iamtypes.Role{
			RoleName: aws.String(name),
			Arn:      aws.String(roleARN(name)),
		})
	}
	if end < len(names) {
		out.IsTruncated = true
		out.Marker = aws.String(names[end-1])
	}
	return out, nil
}

// ListRolePolicies implements the AWS ListRolePolicies API over the fake. Only
// the RoleName input parameter is used. The options are ignored. If the given
// role name has a faultRoleDeletedInline fault, a NoSuchEntity error is
// returned. If it has a faultInlineTransient fault, a different error is
// returned. Otherwise all the inline policies in the fake for the role are
// returned in one go.
func (f *fakeIAMRoles) ListRolePolicies(_ context.Context, in *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	name := aws.ToString(in.RoleName)
	role, ok := f.roles[name]
	if !ok {
		return nil, noSuchRole(name)
	}
	switch role.fault {
	case faultRoleDeletedInline:
		return nil, noSuchRole(name)
	case faultInlineTransient:
		return nil, errTransientIAM
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: slices.Sorted(maps.Keys(role.inline))}, nil
}

// GetRolePolicy implements the AWS GetRolePolicy API over the fake. Only the
// RoleName and PolicyName input parameters are used. The options are ignored.
// If the role does not exist or it has a a fault of faultPolicyDeleted or the
// inline policy does not exist, a NoSuchEntity error is returned. Otherwise
// the policy document in the fake for the role policy is returned in the
// output.
func (f *fakeIAMRoles) GetRolePolicy(_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	roleName, policyName := aws.ToString(in.RoleName), aws.ToString(in.PolicyName)
	role, ok := f.roles[roleName]
	if !ok || role.fault == faultRoleDeletedAfterListPolicies {
		return nil, noSuchRole(roleName)
	}
	if role.fault == faultPolicyDeleted {
		return nil, noSuchRolePolicy(policyName)
	}
	document, ok := role.inline[policyName]
	if !ok {
		return nil, noSuchRolePolicy(policyName)
	}
	return &iam.GetRolePolicyOutput{
		RoleName:       in.RoleName,
		PolicyName:     in.PolicyName,
		PolicyDocument: aws.String(document),
	}, nil
}

// ListAttachedRolePolicies implements the AWS ListAttachedRolePolicies API
// over the fake. Only the RoleName input parameter is used. The options are
// ignored. If the role does not exist or the role has a fault of
// faultRoleDeletedAttached, a NoSuchEntity error is returned. If the role has
// a fault of faultAttachedTransient, a different error is returned. Otherwise
// the attached policy names/ARNs are returned.
func (f *fakeIAMRoles) ListAttachedRolePolicies(_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	name := aws.ToString(in.RoleName)
	role, ok := f.roles[name]
	if !ok {
		return nil, noSuchRole(name)
	}
	switch role.fault {
	case faultRoleDeletedAttached, faultRoleDeletedAfterListPolicies:
		return nil, noSuchRole(name)
	case faultAttachedTransient:
		return nil, errTransientIAM
	}

	out := &iam.ListAttachedRolePoliciesOutput{}
	for _, arn := range role.attached {
		out.AttachedPolicies = append(out.AttachedPolicies, iamtypes.AttachedPolicy{
			PolicyArn:  aws.String(arn),
			PolicyName: aws.String(arn),
		})
	}
	return out, nil
}

func roleARN(name string) string {
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", rolesTestAccountID, name)
}

// noSuchRole matches the error AWS returns when the named role does not exist.
func noSuchRole(name string) error {
	return &iamtypes.NoSuchEntityException{
		Message: aws.String(fmt.Sprintf("The role with name %s cannot be found.", name)),
	}
}

// noSuchRolePolicy matches the error AWS returns when the named inline policy
// does not exist. Note that GetRolePolicy names two entities and returns the
// same error code for both, which is why the fetcher does not try to tell them
// apart.
func noSuchRolePolicy(name string) error {
	return &iamtypes.NoSuchEntityException{
		Message: aws.String(fmt.Sprintf("The role policy with name %s cannot be found.", name)),
	}
}

// pollRoles runs pollAWSRoles against the fake and returns the result along
// with any errors the fetcher reported.
func pollRoles(t *testing.T, fake *fakeIAMRoles, lastResult *Resources) (*Resources, []error) {
	t.Helper()

	if lastResult == nil {
		lastResult = &Resources{}
	}
	fetcher := &Fetcher{
		Config: Config{
			AccountID:         rolesTestAccountID,
			AWSConfigProvider: &mocks.AWSConfigProvider{},
			awsClients:        fakeAWSClients{iamClient: fake},
		},
		lastResult: lastResult,
	}

	var (
		mu   sync.Mutex
		errs []error
	)
	collectErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}

	result := &Resources{}
	require.NoError(t, fetcher.pollAWSRoles(context.Background(), result, collectErr)())
	return result, errs
}

func roleNames(t *testing.T, roles []*accessgraphv1alpha.AWSRoleV1) []string {
	t.Helper()

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		// A nil here means a role was dropped without being compacted out,
		// which would be pushed to the Access Graph as an empty resource.
		require.NotNil(t, role, "result.Roles must not contain nil entries")
		names = append(names, role.GetName())
	}
	slices.Sort(names)
	return names
}

func inlinePolicyNames(policies []*accessgraphv1alpha.AWSRoleInlinePolicyV1) map[string][]string {
	byRole := make(map[string][]string)
	for _, policy := range policies {
		role := policy.GetAwsRole().GetName()
		byRole[role] = append(byRole[role], policy.GetPolicyName())
	}
	for _, names := range byRole {
		slices.Sort(names)
	}
	return byRole
}

func attachedPolicyARNs(attached []*accessgraphv1alpha.AWSRoleAttachedPolicies) map[string][]string {
	byRole := make(map[string][]string)
	for _, entry := range attached {
		role := entry.GetAwsRole().GetName()
		for _, policy := range entry.GetPolicies() {
			byRole[role] = append(byRole[role], policy.GetArn())
		}
	}
	for _, arns := range byRole {
		slices.Sort(arns)
	}
	return byRole
}

func TestPollAWSRoles(t *testing.T) {
	tests := []struct {
		name         string
		roles        map[string]fakeRole
		wantRoles    []string
		wantInline   map[string][]string
		wantAttached map[string][]string
		wantErrs     int
	}{
		{
			name: "roles are enriched with their policies",
			roles: map[string]fakeRole{
				"alpha": {
					inline:   map[string]string{"inline-a": "doc-a", "inline-b": "doc-b"},
					attached: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
				},
				"beta": {
					inline:   map[string]string{"inline-c": "doc-c"},
					attached: []string{"arn:aws:iam::aws:policy/AdministratorAccess"},
				},
			},
			wantRoles: []string{"alpha", "beta"},
			wantInline: map[string][]string{
				"alpha": {"inline-a", "inline-b"},
				"beta":  {"inline-c"},
			},
			wantAttached: map[string][]string{
				"alpha": {"arn:aws:iam::aws:policy/ReadOnlyAccess"},
				"beta":  {"arn:aws:iam::aws:policy/AdministratorAccess"},
			},
		},
		{
			name:      "no roles in the account",
			roles:     map[string]fakeRole{},
			wantRoles: []string{},
		},
		{
			name: "role without policies is still reported",
			roles: map[string]fakeRole{
				"alpha": {},
			},
			wantRoles: []string{"alpha"},
		},
		{
			// A role is deleted after being listed but before listing the
			// inline policies for it. That role should be ignored.
			name: "role deleted before its inline policies are fetched",
			roles: map[string]fakeRole{
				"alpha": {inline: map[string]string{"inline-a": "doc-a"}},
				"gone":  {fault: faultRoleDeletedInline},
			},
			wantRoles:  []string{"alpha"},
			wantInline: map[string][]string{"alpha": {"inline-a"}},
		},
		{
			// A role is deleted after being listed and listing the inline
			// policies for it but before getting the inline policies.
			// That role should be ignored.
			name: "role deleted before its inline policies are fetched",
			roles: map[string]fakeRole{
				"alpha": {inline: map[string]string{"inline-a": "doc-a"}},
				"gone":  {fault: faultRoleDeletedAfterListPolicies},
			},
			wantRoles:  []string{"alpha"},
			wantInline: map[string][]string{"alpha": {"inline-a"}},
		},
		{
			// A role is deleted after being listed but before listing the
			// attached policies for it. That role should be ignored.
			name: "role deleted before its attached policies are fetched",
			roles: map[string]fakeRole{
				"alpha": {inline: map[string]string{"inline-a": "doc-a"}},
				"gone":  {fault: faultRoleDeletedAttached},
			},
			wantRoles:  []string{"alpha"},
			wantInline: map[string][]string{"alpha": {"inline-a"}},
		},
		{
			// A deleted role must contribute no policies, even though its
			// inline policies were fetched successfully first.
			name: "deleted role contributes no policies",
			roles: map[string]fakeRole{
				"gone": {
					inline: map[string]string{"inline-a": "doc-a"},
					fault:  faultRoleDeletedAttached,
				},
			},
			wantRoles: []string{},
		},
		{
			name: "several roles deleted concurrently",
			roles: map[string]fakeRole{
				"a": {fault: faultRoleDeletedInline},
				"b": {inline: map[string]string{"inline-b": "doc-b"}},
				"c": {fault: faultRoleDeletedAttached},
				"d": {fault: faultRoleDeletedInline},
				"e": {inline: map[string]string{"inline-e": "doc-e"}},
				"f": {fault: faultRoleDeletedAttached},
			},
			wantRoles: []string{"b", "e"},
			wantInline: map[string][]string{
				"b": {"inline-b"},
				"e": {"inline-e"},
			},
		},
		{
			// If an inline policy is deleted (returning NoSuchEntity) but the
			// role is not, the role should still be reported without the inline
			// policy, but with attached policies.
			name: "inline policy deleted while the role is being read",
			roles: map[string]fakeRole{
				"alpha": {
					inline:   map[string]string{"inline-a": "doc-a"},
					attached: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
					fault:    faultPolicyDeleted,
				},
			},
			wantRoles: []string{"alpha"},
			wantAttached: map[string][]string{
				"alpha": {"arn:aws:iam::aws:policy/ReadOnlyAccess"},
			},
		},
		{
			// A role that still exists but could not be read is kept, and the
			// failure is reported.
			name: "transient inline failure keeps the role and reports an error",
			roles: map[string]fakeRole{
				"alpha": {
					inline:   map[string]string{"inline-a": "doc-a"},
					attached: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
					fault:    faultInlineTransient,
				},
			},
			wantRoles: []string{"alpha"},
			wantAttached: map[string][]string{
				"alpha": {"arn:aws:iam::aws:policy/ReadOnlyAccess"},
			},
			wantErrs: 1,
		},
		{
			name: "transient attached failure keeps the role and reports an error",
			roles: map[string]fakeRole{
				"alpha": {
					inline: map[string]string{"inline-a": "doc-a"},
					fault:  faultAttachedTransient,
				},
			},
			wantRoles:  []string{"alpha"},
			wantInline: map[string][]string{"alpha": {"inline-a"}},
			wantErrs:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errs := pollRoles(t, &fakeIAMRoles{roles: tt.roles}, nil)

			require.Equal(t, tt.wantRoles, roleNames(t, result.Roles))
			require.Len(t, errs, tt.wantErrs)

			wantInline := tt.wantInline
			if wantInline == nil {
				wantInline = map[string][]string{}
			}
			require.Equal(t, wantInline, inlinePolicyNames(result.RoleInlinePolicies))

			wantAttached := tt.wantAttached
			if wantAttached == nil {
				wantAttached = map[string][]string{}
			}
			require.Equal(t, wantAttached, attachedPolicyARNs(result.RoleAttachedPolicies))
		})
	}
}

// TestPollAWSRolesPolicyContents asserts the policy documents and identifying
// fields survive the round trip, not just the names.
func TestPollAWSRolesPolicyContents(t *testing.T) {
	result, errs := pollRoles(t, &fakeIAMRoles{
		roles: map[string]fakeRole{
			"alpha": {
				inline:   map[string]string{"inline-a": `{"Version":"2012-10-17"}`},
				attached: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
			},
		},
	}, nil)
	require.Empty(t, errs)

	require.Len(t, result.Roles, 1)
	require.Equal(t, "alpha", result.Roles[0].GetName())
	require.Equal(t, roleARN("alpha"), result.Roles[0].GetArn())
	require.Equal(t, rolesTestAccountID, result.Roles[0].GetAccountId())

	require.Len(t, result.RoleInlinePolicies, 1)
	inline := result.RoleInlinePolicies[0]
	require.Equal(t, "inline-a", inline.GetPolicyName())
	require.Equal(t, []byte(`{"Version":"2012-10-17"}`), inline.GetPolicyDocument())
	require.Equal(t, rolesTestAccountID, inline.GetAccountId())
	require.Equal(t, "alpha", inline.GetAwsRole().GetName())

	require.Len(t, result.RoleAttachedPolicies, 1)
	attached := result.RoleAttachedPolicies[0]
	require.Equal(t, rolesTestAccountID, attached.GetAccountId())
	require.Equal(t, "alpha", attached.GetAwsRole().GetName())
	require.Len(t, attached.GetPolicies(), 1)
	require.Equal(t, "arn:aws:iam::aws:policy/ReadOnlyAccess", attached.GetPolicies()[0].GetArn())
}

// TestPollAWSRolesFallsBackToLastResult asserts that a role which still exists
// but could not be read keeps the policies discovered by the previous poll,
// rather than having them reconciled out of the graph.
func TestPollAWSRolesFallsBackToLastResult(t *testing.T) {
	role := accessgraphv1alpha.AWSRoleV1_builder{
		Name:      "alpha",
		Arn:       roleARN("alpha"),
		AccountId: rolesTestAccountID,
	}.Build()

	lastResult := &Resources{
		Roles: []*accessgraphv1alpha.AWSRoleV1{role},
		RoleInlinePolicies: []*accessgraphv1alpha.AWSRoleInlinePolicyV1{
			accessgraphv1alpha.AWSRoleInlinePolicyV1_builder{
				PolicyName:     "previous-inline",
				PolicyDocument: []byte("previous-doc"),
				AccountId:      rolesTestAccountID,
				AwsRole:        role,
			}.Build(),
		},
		RoleAttachedPolicies: []*accessgraphv1alpha.AWSRoleAttachedPolicies{
			accessgraphv1alpha.AWSRoleAttachedPolicies_builder{
				AccountId: rolesTestAccountID,
				AwsRole:   role,
				Policies: []*accessgraphv1alpha.AttachedPolicyV1{
					accessgraphv1alpha.AttachedPolicyV1_builder{
						Arn:        "arn:aws:iam::aws:policy/PreviousAccess",
						PolicyName: "PreviousAccess",
					}.Build(),
				},
			}.Build(),
		},
	}

	tests := []struct {
		name         string
		fault        roleFault
		wantInline   map[string][]string
		wantAttached map[string][]string
	}{
		{
			name:  "inline policies carried over",
			fault: faultInlineTransient,
			// Inline comes from the previous poll, attached from this one.
			wantInline:   map[string][]string{"alpha": {"previous-inline"}},
			wantAttached: map[string][]string{"alpha": {"arn:aws:iam::aws:policy/CurrentAccess"}},
		},
		{
			name:         "attached policies carried over",
			fault:        faultAttachedTransient,
			wantInline:   map[string][]string{"alpha": {"current-inline"}},
			wantAttached: map[string][]string{"alpha": {"arn:aws:iam::aws:policy/PreviousAccess"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errs := pollRoles(t, &fakeIAMRoles{
				roles: map[string]fakeRole{
					"alpha": {
						inline:   map[string]string{"current-inline": "current-doc"},
						attached: []string{"arn:aws:iam::aws:policy/CurrentAccess"},
						fault:    tt.fault,
					},
				},
			}, lastResult)

			require.Len(t, errs, 1)
			require.Equal(t, []string{"alpha"}, roleNames(t, result.Roles))
			require.Equal(t, tt.wantInline, inlinePolicyNames(result.RoleInlinePolicies))
			require.Equal(t, tt.wantAttached, attachedPolicyARNs(result.RoleAttachedPolicies))
		})
	}
}

// TestPollAWSRolesFallbackWithoutPreviousResult asserts a transient failure on
// the first ever poll degrades to no policies rather than failing the sync.
func TestPollAWSRolesFallbackWithoutPreviousResult(t *testing.T) {
	result, errs := pollRoles(t, &fakeIAMRoles{
		roles: map[string]fakeRole{
			"alpha": {
				inline: map[string]string{"inline-a": "doc-a"},
				fault:  faultInlineTransient,
			},
		},
	}, nil)

	require.Len(t, errs, 1)
	require.Equal(t, []string{"alpha"}, roleNames(t, result.Roles))
	require.Empty(t, result.RoleInlinePolicies)
}

// TestPollAWSRolesListRolesFailure asserts that when the role listing itself
// fails, the previous poll's roles and role policies are preserved verbatim.
func TestPollAWSRolesListRolesFailure(t *testing.T) {
	role := accessgraphv1alpha.AWSRoleV1_builder{
		Name:      "alpha",
		Arn:       roleARN("alpha"),
		AccountId: rolesTestAccountID,
	}.Build()

	lastResult := &Resources{
		Roles: []*accessgraphv1alpha.AWSRoleV1{role},
		RoleInlinePolicies: []*accessgraphv1alpha.AWSRoleInlinePolicyV1{
			accessgraphv1alpha.AWSRoleInlinePolicyV1_builder{
				PolicyName: "previous-inline",
				AccountId:  rolesTestAccountID,
				AwsRole:    role,
			}.Build(),
		},
		RoleAttachedPolicies: []*accessgraphv1alpha.AWSRoleAttachedPolicies{
			accessgraphv1alpha.AWSRoleAttachedPolicies_builder{
				AccountId: rolesTestAccountID,
				AwsRole:   role,
				Policies: []*accessgraphv1alpha.AttachedPolicyV1{
					accessgraphv1alpha.AttachedPolicyV1_builder{
						Arn:        "arn:aws:iam::aws:policy/PreviousAccess",
						PolicyName: "PreviousAccess",
					}.Build(),
				},
			}.Build(),
		},
		// Group state must not be disturbed by the role poller.
		GroupInlinePolicies: []*accessgraphv1alpha.AWSGroupInlinePolicyV1{
			accessgraphv1alpha.AWSGroupInlinePolicyV1_builder{
				PolicyName: "group-inline",
				AccountId:  rolesTestAccountID,
			}.Build(),
		},
	}

	result, errs := pollRoles(t, &fakeIAMRoles{listRolesErr: errTransientIAM}, lastResult)

	require.Len(t, errs, 1)
	require.ErrorIs(t, errs[0], errTransientIAM)

	require.Equal(t, lastResult.Roles, result.Roles)
	require.Equal(t, lastResult.RoleInlinePolicies, result.RoleInlinePolicies)
	require.Equal(t, lastResult.RoleAttachedPolicies, result.RoleAttachedPolicies)

	// The role poller must not write to the group fields.
	require.Empty(t, result.GroupInlinePolicies)
	require.Empty(t, result.GroupAttachedPolicies)
}

// TestPollAWSRolesPagination asserts every page of roles is enriched, not just
// the first.
func TestPollAWSRolesPagination(t *testing.T) {
	roles := map[string]fakeRole{}
	want := make([]string, 0, 7)
	for i := range 7 {
		name := fmt.Sprintf("role-%d", i)
		roles[name] = fakeRole{inline: map[string]string{"inline-" + name: "doc"}}
		want = append(want, name)
	}
	slices.Sort(want)

	result, errs := pollRoles(t, &fakeIAMRoles{roles: roles, maxPageSize: 2}, nil)

	require.Empty(t, errs)
	require.Equal(t, want, roleNames(t, result.Roles))
	require.Len(t, result.RoleInlinePolicies, len(want))
}
