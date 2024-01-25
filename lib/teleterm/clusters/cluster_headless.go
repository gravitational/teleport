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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
)

// WatchPendingHeadlessAuthentications watches the backend for pending headless authentication requests for the user.
func (c *Cluster) WatchPendingHeadlessAuthentications(ctx context.Context) (watcher types.Watcher, close func(), err error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	rootClient, err := proxyClient.ConnectToRootCluster(ctx)
	if err != nil {
		proxyClient.Close()
		return nil, nil, trace.Wrap(err)
	}

	watcher, err = rootClient.WatchPendingHeadlessAuthentications(ctx)
	if err != nil {
		proxyClient.Close()
		rootClient.Close()
		return nil, nil, trace.Wrap(err)
	}

	close = func() {
		watcher.Close()
		proxyClient.Close()
		rootClient.Close()
	}

	return watcher, close, trace.Wrap(err)
}

// WatchHeadlessAuthentications watches the backend for headless authentication events for the user.
func (c *Cluster) WatchHeadlessAuthentications(ctx context.Context) (watcher types.Watcher, close func(), err error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	rootClient, err := proxyClient.ConnectToRootCluster(ctx)
	if err != nil {
		proxyClient.Close()
		return nil, nil, trace.Wrap(err)
	}

	watch := types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindHeadlessAuthentication,
			Filter: (&types.HeadlessAuthenticationFilter{
				Username: c.clusterClient.Username,
			}).IntoMap(),
		}},
	}

	watcher, err = rootClient.NewWatcher(ctx, watch)
	if err != nil {
		proxyClient.Close()
		rootClient.Close()
		return nil, nil, trace.Wrap(err)
	}

	close = func() {
		watcher.Close()
		proxyClient.Close()
		rootClient.Close()
	}

	return watcher, close, trace.Wrap(err)
}

// UpdateHeadlessAuthenticationState updates the headless authentication matching the given id to the given state.
// MFA will be prompted when updating to the approve state.
func (c *Cluster) UpdateHeadlessAuthenticationState(ctx context.Context, headlessID string, state types.HeadlessAuthenticationState) error {
	err := AddMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		rootClient, err := proxyClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		// If changing state to approved, create an MFA challenge and prompt for MFA.
		var mfaResponse *proto.MFAAuthenticateResponse
		if state == types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED {
			chall, err := rootClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
					ContextUser: &proto.ContextUser{},
				},
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN,
				},
			})
			if err != nil {
				return trace.Wrap(err)
			}

			mfaResponse, err = c.clusterClient.PromptMFA(ctx, chall)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		err = rootClient.UpdateHeadlessAuthenticationState(ctx, headlessID, state, mfaResponse)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}
