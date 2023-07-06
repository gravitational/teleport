// Copyright 2022 Gravitational, Inc
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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// WatchPendingHeadlessAuthentications watches the backend for pending headless authentication requests for the user.
func (c *Cluster) WatchPendingHeadlessAuthentications(ctx context.Context) (watcher types.Watcher, close func(), err error) {
	err = addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		rootClient, err := proxyClient.ConnectToRootCluster(ctx)
		if err != nil {
			proxyClient.Close()
			return trace.Wrap(err)
		}

		watcher, err = rootClient.WatchPendingHeadlessAuthentications(ctx)
		if err != nil {
			proxyClient.Close()
			rootClient.Close()
			return trace.Wrap(err)
		}

		close = func() {
			watcher.Close()
			proxyClient.Close()
			rootClient.Close()
		}

		return nil
	})
	return watcher, close, trace.Wrap(err)
}

// UpdateHeadlessAuthenticationState updates the headless authentication matching the given id to the given state.
// MFA will be prompted when updating to the approve state.
func (c *Cluster) UpdateHeadlessAuthenticationState(ctx context.Context, headlessID string, state types.HeadlessAuthenticationState) error {
	err := addMetadataToRetryableError(ctx, func() error {
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
			})
			if err != nil {
				return trace.Wrap(err)
			}

			mfaResponse, err = c.clusterClient.PromptMFAChallenge(ctx, c.clusterClient.WebProxyAddr, chall, nil)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		err = rootClient.UpdateHeadlessAuthenticationState(ctx, headlessID, state, mfaResponse)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}
