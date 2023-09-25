/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mongodb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas-sdk/v20230201008/admin"

	"github.com/gravitational/teleport/lib/asciitable"
)

func TestSDK(t *testing.T) {
	apiKey := os.Getenv("MONGODB_ATLAS_PUBLIC_KEY")
	apiSecret := os.Getenv("MONGODB_ATLAS_PRIVATE_KEY")

	sdk, err := admin.NewClient(admin.UseDigestAuth(apiKey, apiSecret))
	require.NoError(t, err)

	project := "Project 0"
	groupID := mustGetProjectGroupID(t, sdk, project)
	mustListAndPrintCustomRoles(t, sdk, groupID)
	mustListAndPrintDatabaseUsers(t, sdk, groupID)
	mustUpdateUserRoles(t, sdk, groupID, "CN=db-reader", []string{"read@test2", "readTest"})
}

func mustGetProjectGroupID(t *testing.T, sdk *admin.APIClient, wantProject string) string {
	t.Helper()
	projects, response, err := sdk.ProjectsApi.ListProjects(context.Background()).Execute()
	require.NoError(t, err)
	defer response.Body.Close()

	for _, project := range projects.Results {
		if project.Name == wantProject {
			return project.GetId()
		}
	}
	require.Failf(t, "cannot find project %v", wantProject)
	return ""
}

func mustListAndPrintCustomRoles(t *testing.T, sdk *admin.APIClient, groupID string) {
	t.Helper()
	roles, response, err := sdk.CustomDatabaseRolesApi.ListCustomDatabaseRoles(context.Background(), groupID).Execute()
	require.NoError(t, err)
	defer response.Body.Close()

	table := asciitable.MakeTable([]string{"Role Name", "Inherited Roles", "Actions"})
	for _, role := range roles {
		table.AddRow([]string{
			role.GetRoleName(),
			fmt.Sprintf("%+v", role.InheritedRoles),
			fmt.Sprintf("%+v", role.Actions),
		})
	}
	t.Logf("Listing users:\n" + table.AsBuffer().String())
}

func mustListAndPrintDatabaseUsers(t *testing.T, sdk *admin.APIClient, groupID string) {
	t.Helper()
	users, response, err := sdk.DatabaseUsersApi.ListDatabaseUsers(context.Background(), groupID).Execute()
	require.NoError(t, err)
	defer response.Body.Close()
	printUsers(t, users.Results)
}

func printUsers(t *testing.T, users []admin.CloudDatabaseUser) {
	table := asciitable.MakeTable([]string{"Username", "Roles", "Auth Type", "Database Name"})
	for _, user := range users {
		table.AddRow([]string{
			user.GetUsername(),
			strings.Join(roleNames(user.Roles), ","),
			authType(&user),
			user.GetDatabaseName(),
		})
	}
	t.Logf("Listing users:\n" + table.AsBuffer().String())
}

func mustUpdateUserRoles(t *testing.T, sdk *admin.APIClient, groupID, username string, roles []string) {
	t.Helper()
	user, response, err := sdk.DatabaseUsersApi.GetDatabaseUser(
		context.Background(),
		groupID,
		"$external",
		username,
	).Execute()
	require.NoError(t, err)
	response.Body.Close()

	user.Roles = make([]admin.DatabaseUserRole, 0, len(roles))
	for _, role := range roles {
		switch strings.Count(role, "@") {
		case 0:
			user.Roles = append(user.Roles, admin.DatabaseUserRole{
				DatabaseName: "admin",
				RoleName:     role,
			})
		case 1:
			roleName, databaseName, ok := strings.Cut(role, "@")
			require.True(t, ok)
			user.Roles = append(user.Roles, admin.DatabaseUserRole{
				DatabaseName: databaseName,
				RoleName:     roleName,
			})
		default:
			require.Failf(t, "invalid role: %v", role)
		}
	}

	user, response, err = sdk.DatabaseUsersApi.UpdateDatabaseUser(
		context.Background(),
		groupID,
		"$external",
		username,
		user,
	).Execute()
	require.NoError(t, err)
	response.Body.Close()
	printUsers(t, []admin.CloudDatabaseUser{*user})
}

func roleNames(roles []admin.DatabaseUserRole) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, fmt.Sprintf("%v@%v", role.GetRoleName(), role.GetDatabaseName()))
	}
	return names
}

func authType(user *admin.CloudDatabaseUser) string {
	switch {
	case user.GetAwsIAMType() != "NONE":
		return "AWS IAM"
	case user.GetX509Type() != "NONE":
		return "X.509"
	default:
		return "others"
	}
}
