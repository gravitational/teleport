/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestTrimDurationSuffix(t *testing.T) {
	t.Parallel()
	var testCases = []struct {
		comment string
		ts      time.Duration
		wantFmt string
	}{
		{
			comment: "trim minutes/seconds",
			ts:      1 * time.Hour,
			wantFmt: "1h",
		},
		{
			comment: "trim seconds",
			ts:      1 * time.Minute,
			wantFmt: "1m",
		},
		{
			comment: "does not trim non-zero suffix",
			ts:      90 * time.Second,
			wantFmt: "1m30s",
		},
		{
			comment: "does not trim zero in the middle",
			ts:      3630 * time.Second,
			wantFmt: "1h0m30s",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.comment, func(t *testing.T) {
			fmt := trimDurationZeroSuffix(tt.ts)
			require.Equal(t, tt.wantFmt, fmt)
		})
	}
}

func TestUserAdd(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	ctx := context.Background()
	client := testenv.MakeDefaultAuthClient(t, process)

	tests := []struct {
		name string
		args []string
		// if dontAddDefaultRoles is false (default), `--roles auditor` is added to the command line.
		// this makes it easier to see the essence of particular test case.
		dontAddDefaultRoles bool
		wantRoles           []string
		wantTraits          map[string][]string
		errorChecker        require.ErrorAssertionFunc
	}{
		{
			name:                "set roles",
			dontAddDefaultRoles: true,
			args:                []string{"--roles", "editor,access"},
			wantRoles:           []string{"editor", "access"},
		},
		{
			name:                "nonexistent roles",
			dontAddDefaultRoles: true,
			args:                []string{"--roles", "editor,access,fake"},
			errorChecker: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), err)
			},
		},
		{
			name: "logins",
			args: []string{"--logins", "l1,l2,l3"},
			wantTraits: map[string][]string{
				constants.TraitLogins: {"l1", "l2", "l3"},
			},
		},
		{
			name: "windows logins",
			args: []string{"--windows-logins", "w1,w2,w3"},
			wantTraits: map[string][]string{
				constants.TraitWindowsLogins: {"w1", "w2", "w3"},
			},
		},
		{
			name: "kube users",
			args: []string{"--kubernetes-users", "k1,k2,k3"},
			wantTraits: map[string][]string{
				constants.TraitKubeUsers: {"k1", "k2", "k3"},
			},
		},
		{
			name: "kube groups",
			args: []string{"--kubernetes-groups", "k4,k5,k6"},
			wantTraits: map[string][]string{
				constants.TraitKubeGroups: {"k4", "k5", "k6"},
			},
		},
		{
			name: "db users",
			args: []string{"--db-users", "d1,d2,d3"},
			wantTraits: map[string][]string{
				constants.TraitDBUsers: {"d1", "d2", "d3"},
			},
		},
		{
			name: "db names",
			args: []string{"--db-names", "d4,d5,d6"},
			wantTraits: map[string][]string{
				constants.TraitDBNames: {"d4", "d5", "d6"},
			},
		},
		{
			name: "AWS role ARNs",
			args: []string{"--aws-role-arns", "a1,a2,a3"},
			wantTraits: map[string][]string{
				constants.TraitAWSRoleARNs: {"a1", "a2", "a3"},
			},
		},
		{
			name: "Azure identities",
			args: []string{"--azure-identities", "/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-1,/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-2,/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-3"},
			wantTraits: map[string][]string{
				constants.TraitAzureIdentities: {
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-1",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-2",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-3",
				},
			},
		},
		{
			name: "GCP service accounts",
			args: []string{"--gcp-service-accounts", "a1@example-123456.iam.gserviceaccount.com,a2@example-456789.iam.gserviceaccount.com,a3@example-111222.iam.gserviceaccount.com"},
			wantTraits: map[string][]string{
				constants.TraitGCPServiceAccounts: {
					"a1@example-123456.iam.gserviceaccount.com",
					"a2@example-456789.iam.gserviceaccount.com",
					"a3@example-111222.iam.gserviceaccount.com",
				},
			},
		},
		{
			name: "invalid GCP service account are rejected",
			args: []string{"--gcp-service-accounts", "foobar"},
			errorChecker: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "GCP service account \"foobar\" is invalid")
			},
		},
	}

	for ix, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			username := fmt.Sprintf("test-user-%v", ix)

			args := []string{"add"}
			if !tc.dontAddDefaultRoles {
				args = append(args, "--roles", "auditor")
			}
			args = append(args, tc.args...)
			args = append(args, username)
			err := runUserCommand(t, client, args)
			if tc.errorChecker != nil {
				tc.errorChecker(t, err)
				return
			}

			require.NoError(t, err)
			createdUser, err := client.GetUser(ctx, username, false)
			require.NoError(t, err)

			if len(tc.wantRoles) > 0 {
				require.Equal(t, tc.wantRoles, createdUser.GetRoles())
			}

			for trait, values := range tc.wantTraits {
				require.Equal(t, values, createdUser.GetTraits()[trait])
			}
		})
	}
}

func TestUserUpdate(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	ctx := context.Background()
	client := testenv.MakeDefaultAuthClient(t, process)

	baseUser, err := types.NewUser("test-user")
	require.NoError(t, err)

	tests := []struct {
		name         string
		args         []string
		wantRoles    []string
		wantTraits   map[string][]string
		errorChecker require.ErrorAssertionFunc
	}{
		{
			name: "no args",
			errorChecker: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), err)
			},
		},
		{
			name:      "new roles",
			args:      []string{"--set-roles", "editor,access"},
			wantRoles: []string{"editor", "access"},
		},
		{
			name: "nonexistent roles",
			args: []string{"--set-roles", "editor,access,fake"},
			errorChecker: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), err)
			},
		},
		{
			name: "new logins",
			args: []string{"--set-logins", "l1,l2,l3"},
			wantTraits: map[string][]string{
				constants.TraitLogins: {"l1", "l2", "l3"},
			},
		},
		{
			name: "new windows logins",
			args: []string{"--set-windows-logins", "w1,w2,w3"},
			wantTraits: map[string][]string{
				constants.TraitWindowsLogins: {"w1", "w2", "w3"},
			},
		},
		{
			name: "new kube users",
			args: []string{"--set-kubernetes-users", "k1,k2,k3"},
			wantTraits: map[string][]string{
				constants.TraitKubeUsers: {"k1", "k2", "k3"},
			},
		},
		{
			name: "new kube groups",
			args: []string{"--set-kubernetes-groups", "k4,k5,k6"},
			wantTraits: map[string][]string{
				constants.TraitKubeGroups: {"k4", "k5", "k6"},
			},
		},
		{
			name: "new db users",
			args: []string{"--set-db-users", "d1,d2,d3"},
			wantTraits: map[string][]string{
				constants.TraitDBUsers: {"d1", "d2", "d3"},
			},
		},
		{
			name: "new db names",
			args: []string{"--set-db-names", "d4,d5,d6"},
			wantTraits: map[string][]string{
				constants.TraitDBNames: {"d4", "d5", "d6"},
			},
		},
		{
			name: "new db roles",
			args: []string{"--set-db-roles", "d7,d8,d9"},
			wantTraits: map[string][]string{
				constants.TraitDBRoles: {"d7", "d8", "d9"},
			},
		},
		{
			name: "new AWS role ARNs",
			args: []string{"--set-aws-role-arns", "a1,a2,a3"},
			wantTraits: map[string][]string{
				constants.TraitAWSRoleARNs: {"a1", "a2", "a3"},
			},
		},
		{
			name: "new Azure identities",
			args: []string{"--set-azure-identities", "/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-1,/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-2,/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-3"},
			wantTraits: map[string][]string{
				constants.TraitAzureIdentities: {
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-1",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-2",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure-3",
				},
			},
		},
		{
			name: "new GCP service accounts",
			args: []string{"--set-gcp-service-accounts", "a1@example-123456.iam.gserviceaccount.com,a2@example-456789.iam.gserviceaccount.com,a3@example-111222.iam.gserviceaccount.com"},
			wantTraits: map[string][]string{
				constants.TraitGCPServiceAccounts: {
					"a1@example-123456.iam.gserviceaccount.com",
					"a2@example-456789.iam.gserviceaccount.com",
					"a3@example-111222.iam.gserviceaccount.com",
				},
			},
		},
		{
			name: "invalid GCP service account are rejected",
			args: []string{"--set-gcp-service-accounts", "foobar"},
			errorChecker: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "GCP service account \"foobar\" is invalid")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err = client.UpsertUser(ctx, baseUser)
			require.NoError(t, err)
			args := append([]string{"update"}, tc.args...)
			args = append(args, "test-user")
			err := runUserCommand(t, client, args)
			if tc.errorChecker != nil {
				tc.errorChecker(t, err)
				return
			}

			require.NoError(t, err)
			updatedUser, err := client.GetUser(ctx, "test-user", false)
			require.NoError(t, err)

			if len(tc.wantRoles) > 0 {
				require.Equal(t, tc.wantRoles, updatedUser.GetRoles())
			}

			for trait, values := range tc.wantTraits {
				require.Equal(t, values, updatedUser.GetTraits()[trait])
			}
		})
	}
}
