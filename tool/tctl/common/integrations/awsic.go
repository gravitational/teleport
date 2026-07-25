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

package integrations

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

type awsicAccount struct {
	name     string
	permSets []string
}

// listAWSICAccounts prints the AWS Identity Center accounts synced into the
// cluster along with their permission sets.
func (c *Command) listAWSICAccounts(ctx context.Context, clt *authclient.Client) error {
	assignments, err := getAllAWSICAccountAssignments(ctx, clt)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.writeAWSICOutput(assignments))
}

func getAllAWSICAccountAssignments(ctx context.Context, clt *authclient.Client) ([]*identitycenterv1.AccountAssignment, error) {
	resources, err := apiclient.GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
		ResourceType: types.KindIdentityCenterAccountAssignment,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accAssignments := make([]*identitycenterv1.AccountAssignment, 0, len(resources))
	for _, resource := range resources {
		unwrapper, ok := resource.(types.Resource153UnwrapperT[*identitycenterv1.AccountAssignment])
		if !ok {
			return nil, trace.BadParameter("expected account assignment type, got %T", resource)
		}
		accAssignments = append(accAssignments, unwrapper.UnwrapT())
	}
	return accAssignments, nil
}

func (c *Command) writeAWSICOutput(assignments []*identitycenterv1.AccountAssignment) error {
	switch c.awsicAccountsLsFormat {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(c.Stdout, awsicResources(assignments)))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.Stdout, awsicResources(assignments)))
	default:
		return trace.Wrap(c.writeAWSICText(assignments))
	}
}

func (c *Command) writeAWSICText(assignments []*identitycenterv1.AccountAssignment) error {
	accountIDs := []string{}
	accountLookup := make(map[string]*awsicAccount)

	for _, assignment := range assignments {
		spec := assignment.GetSpec()
		accountID := spec.GetAccountId()
		ps := spec.GetPermissionSet()

		account, ok := accountLookup[accountID]
		if !ok {
			account = &awsicAccount{name: spec.GetAccountName()}
			accountLookup[accountID] = account
			accountIDs = append(accountIDs, accountID)
		}
		account.permSets = append(account.permSets, fmt.Sprintf("%s (%s)", ps.GetName(), ps.GetArn()))
	}

	headers := []string{"Account Name", "Account ID", "Permission Sets"}
	rows := make([][]string, 0, len(assignments))

	for _, accountID := range accountIDs {
		account := accountLookup[accountID]
		// Print each permission set in its own row.
		for i, permSet := range account.permSets {
			if i == 0 {
				rows = append(rows, []string{account.name, accountID, permSet})
			} else {
				rows = append(rows, []string{"", "", permSet})
			}
		}
	}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(c.Stdout)
	return trace.Wrap(err)
}

func awsicResources(assignments []*identitycenterv1.AccountAssignment) []types.Resource {
	r := make([]types.Resource, len(assignments))
	for i, assignment := range assignments {
		r[i] = types.ProtoResource153ToLegacy(assignment)
	}
	return r
}
