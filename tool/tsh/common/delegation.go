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
	"errors"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
)

func onDelegationCreateSession(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if cf.Username == "" {
		cf.Username = tc.Username
	}

	req, err := buildCreateDelegationSessionRequest(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var session *delegationv1pb.DelegationSession
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
			ctx := cf.Context

			mfaResp, err := mfa.PerformAdminActionMFACeremony(ctx, clt.PerformMFACeremony, false)
			if err == nil {
				ctx = mfa.ContextWithMFAResponse(ctx, mfaResp)
			} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
				return trace.Wrap(err)
			}

			session, err = clt.DelegationSessionServiceClient().CreateDelegationSession(ctx, req)
			return trace.Wrap(err)
		})
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintf(cf.Stdout(), "Delegation session created: %s\n", session.GetMetadata().GetName())
	return trace.Wrap(err)
}

func buildCreateDelegationSessionRequest(cf *CLIConf) (*delegationv1pb.CreateDelegationSessionRequest, error) {
	if cf.SessionTTL <= 0 {
		return nil, trace.BadParameter("--session-ttl must be greater than zero")
	}

	resources, err := buildDelegationResources(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(cf.DelegationBots) == 0 {
		return nil, trace.BadParameter("at least one --bot must be provided")
	}

	authorizedUsers := make([]*delegationv1pb.DelegationUserSpec, 0, len(cf.DelegationBots))
	for _, botName := range cf.DelegationBots {
		if botName == "" {
			return nil, trace.BadParameter("--bot must not be empty")
		}

		authorizedUsers = append(authorizedUsers, &delegationv1pb.DelegationUserSpec{
			Kind: types.KindBot,
			Matcher: &delegationv1pb.DelegationUserSpec_BotName{
				BotName: botName,
			},
		})
	}

	return &delegationv1pb.CreateDelegationSessionRequest{
		Spec: &delegationv1pb.DelegationSessionSpec{
			User:            cf.Username,
			Resources:       resources,
			AuthorizedUsers: authorizedUsers,
		},
		Ttl: durationpb.New(cf.SessionTTL),
	}, nil
}

func buildDelegationResources(cf *CLIConf) ([]*delegationv1pb.DelegationResourceSpec, error) {
	explicitResources := len(cf.DelegationAllowNodes) +
		len(cf.DelegationAllowDatabases) +
		len(cf.DelegationAllowApps) +
		len(cf.DelegationAllowKubeClusters) +
		len(cf.DelegationAllowWindowsDesktops) +
		len(cf.DelegationAllowGitServers)

	switch {
	case cf.DelegationAllowAll && explicitResources != 0:
		return nil, trace.BadParameter("--allow-all is mutually exclusive with the other --allow-* flags")
	case cf.DelegationAllowAll:
		return []*delegationv1pb.DelegationResourceSpec{{
			Kind: types.Wildcard,
			Name: types.Wildcard,
		}}, nil
	case explicitResources == 0:
		return nil, trace.BadParameter("at least one resource must be provided via --allow-all or an --allow-* flag")
	}

	resources := make([]*delegationv1pb.DelegationResourceSpec, 0, explicitResources)
	for _, name := range cf.DelegationAllowNodes {
		if name == "" {
			return nil, trace.BadParameter("--allow-node must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindNode,
			Name: name,
		})
	}
	for _, name := range cf.DelegationAllowDatabases {
		if name == "" {
			return nil, trace.BadParameter("--allow-db must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindDatabase,
			Name: name,
		})
	}
	for _, name := range cf.DelegationAllowApps {
		if name == "" {
			return nil, trace.BadParameter("--allow-app must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindApp,
			Name: name,
		})
	}
	for _, name := range cf.DelegationAllowKubeClusters {
		if name == "" {
			return nil, trace.BadParameter("--allow-kube-cluster must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindKubernetesCluster,
			Name: name,
		})
	}
	for _, name := range cf.DelegationAllowWindowsDesktops {
		if name == "" {
			return nil, trace.BadParameter("--allow-windows-desktop must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindWindowsDesktop,
			Name: name,
		})
	}
	for _, name := range cf.DelegationAllowGitServers {
		if name == "" {
			return nil, trace.BadParameter("--allow-git-server must not be empty")
		}
		resources = append(resources, &delegationv1pb.DelegationResourceSpec{
			Kind: types.KindGitServer,
			Name: name,
		})
	}

	return resources, nil
}
