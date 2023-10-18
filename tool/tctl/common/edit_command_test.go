// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
)

func TestEditGithubConnector(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()
	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)
	expected, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURL:  "https://proxy.example.com/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "acme",
				Team:         "users",
				Roles:        []string{"access", "editor", "auditor"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")
	created, err := rootClient.CreateGithubConnector(ctx, expected.(*types.GithubConnectorV3))
	require.NoError(t, err, "persisting initial connector resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetClientID("abcdef")

		collection := &connectorsCollection{github: []types.GithubConnector{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())

	}

	// Edit the connector and validate that the expected field is updated.
	_, err = runEditCommand(t, fc, []string{"edit", "connector/github"}, withEditor(editor))
	require.NoError(t, err, "expected editing github connector to succeed")

	actual, err := rootClient.GetGithubConnector(ctx, expected.GetName(), false)
	require.NoError(t, err, "retrieving github connector after edit")
	require.Empty(t, cmp.Diff(created, actual, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.GithubConnectorSpecV3{}, "ClientID", "ClientSecret"),
	))
	require.NotEqual(t, created.GetClientID(), actual.GetClientID(), "client id should have been modified by edit")
	require.Equal(t, expected.GetClientID(), actual.GetClientID(), "client id should match the retrieved connector")

	// Try editing the connector a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, fc, []string{"edit", "connector/github"}, withEditor(editor))
	assert.Error(t, err, "stale connector was allowed to be updated")
	require.Error(t, backend.ErrIncorrectRevision, err, "expected an incorrect revision error got %T", err)
}
