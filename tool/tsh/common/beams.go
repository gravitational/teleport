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

package common

import (
	"context"
	"fmt"
	"time"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

const (
	beamAddPollInterval = 500 * time.Millisecond
	beamAddPollTimeout  = 5 * time.Minute
)

func onBeamsAdd(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AllowHeadless = true

	var beamID string
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		beam, err := clusterClient.AuthClient.BeamsServiceClient().CreateBeam(cf.Context, &beamsv1.CreateBeamRequest{})
		if err != nil {
			return trace.Wrap(err)
		}
		beamID = beam.GetId()
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Beam created: %s\n", beamID)

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	tc.Host = ""
	tc.Labels = nil
	tc.SearchKeywords = nil
	tc.PredicateExpression = fmt.Sprintf(`labels["teleport.internal/beam/id"]==%q`, beamID)
	tc.HostLogin = "root"

	target, err := waitForBeamNode(cf.Context, tc, clusterClient.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.Stdin = cf.Stdin()
	sshFunc := func() error {
		return tc.SSH(cf.Context, nil, client.WithHostAddress(target.Addr))
	}
	if !cf.Relogin {
		return trace.Wrap(sshFunc())
	}
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, sshFunc))
}

func waitForBeamNode(ctx context.Context, tc *client.TeleportClient, authClient authclient.ClientI) (*client.TargetNode, error) {
	pollCtx, cancel := context.WithTimeout(ctx, beamAddPollTimeout)
	defer cancel()

	ticker := time.NewTicker(beamAddPollInterval)
	defer ticker.Stop()

	for {
		page, _, err := apiclient.GetUnifiedResourcePage(pollCtx, authClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{types.KindNode},
			SortBy:              types.SortBy{Field: types.ResourceMetadataName},
			PredicateExpression: tc.PredicateExpression,
			Limit:               1,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(page) > 0 {
			node, ok := page[0].ResourceWithLabels.(types.Server)
			if !ok {
				return nil, trace.BadParameter("expected node resource, got %T", page[0].ResourceWithLabels)
			}
			return &client.TargetNode{
				Hostname: node.GetHostname(),
				Addr:     node.GetName() + ":0",
			}, nil
		}

		select {
		case <-ticker.C:
		case <-pollCtx.Done():
			return nil, trace.LimitExceeded("timed out waiting for beam node to register")
		}
	}
}
