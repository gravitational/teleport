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
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// WatchPendingHeadlessAuthentications watches the backend for pending headless authentication requests for the user.
func (c *Cluster) WatchPendingHeadlessAuthentications(ctx context.Context, rootAuthClient authclient.ClientI) (watcher types.Watcher, close func(), err error) {
	watcher, err = rootAuthClient.WatchPendingHeadlessAuthentications(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	close = func() {
		watcher.Close()
	}

	return watcher, close, trace.Wrap(err)
}

// WatchHeadlessAuthentications watches the backend for headless authentication events for the user.
func (c *Cluster) WatchHeadlessAuthentications(ctx context.Context, rootAuthClient authclient.ClientI) (watcher types.Watcher, close func(), err error) {
	watch := types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindHeadlessAuthentication,
			Filter: (&types.HeadlessAuthenticationFilter{
				Username: c.clusterClient.Username,
			}).IntoMap(),
		}},
	}

	watcher, err = rootAuthClient.NewWatcher(ctx, watch)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	close = func() {
		watcher.Close()
	}

	return watcher, close, trace.Wrap(err)
}

// UpdateHeadlessAuthenticationState updates the headless authentication matching the given id to the given state.
// MFA will be prompted when updating to the approve state.
func (c *Cluster) UpdateHeadlessAuthenticationState(ctx context.Context, rootAuthClient authclient.ClientI, headlessID string, state types.HeadlessAuthenticationState) error {
	err := AddMetadataToRetryableError(ctx, func() error {
		// If changing state to approved, create an MFA challenge and prompt for MFA.
		var mfaResponse *proto.MFAAuthenticateResponse
		var err error
		if state == types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED {
			mfaResponse, err = c.clusterClient.NewMFACeremony().Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN,
				},
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		err = rootAuthClient.UpdateHeadlessAuthenticationState(ctx, headlessID, state, mfaResponse)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}
