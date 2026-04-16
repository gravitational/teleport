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

package srv

import (
	"cmp"
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
	"github.com/gravitational/teleport/lib/sshca"
)

// KeyboardInteractiveAuth performs keyboard-interactive authentication based on the provided preconditions. If further
// authentication is required, it returns a PartialSuccessError containing the necessary SSH server auth callback that
// the SSH server can use to continue the authentication process.
func (h *AuthHandlers) KeyboardInteractiveAuth(
	ctx context.Context,
	preconds []*decisionpb.Precondition,
	id *sshca.Identity,
	perms *ssh.Permissions,
) (*ssh.Permissions, error) {
	// Source cluster must be the cluster the user will perform the MFA ceremony with. This is usually the cluster the
	// user is trying to access, but in some cases, such as trusted clusters, the user has to perform the MFA ceremony
	// with the root cluster instead. In those cases, the RouteToCluster field will be set to the root cluster, so we
	// should use that if it's set.
	sourceCluster := cmp.Or(id.RouteToCluster, id.ClusterName)
	if sourceCluster == "" {
		return nil, trace.BadParameter("identity missing cluster name (this is a bug)")
	}

	// keyboardInteractiveCallback handles keyboard-interactive authentication for modern clients.
	keyboardInteractiveCallback := func(metadata ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
		var verifiers []srvssh.PromptVerifier

		for _, p := range preconds {
			switch p.GetKind() {
			case decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA:
				verifier, err := srvssh.NewMFAPromptVerifier(h.c.ValidatedMFAChallengeVerifier, sourceCluster, id.Username, metadata.SessionID())
				if err != nil {
					return nil, trace.Wrap(err)
				}
				verifiers = append(verifiers, verifier)

				// No default case needed because ensureSupportedPreconditions() was called earlier and would have
				// returned an error early for unexpected preconditions.
			}
		}

		return srvssh.KeyboardInteractiveCallback(
			ctx,
			srvssh.KeyboardInteractiveCallbackParams{
				Metadata:        metadata,
				Challenge:       challenge,
				Permissions:     perms,
				PromptVerifiers: verifiers,
			},
		)
	}

	// Return the PartialSuccessError to indicate that further authentication is required to complete SSH authentication.
	return nil, &ssh.PartialSuccessError{
		Next: ssh.ServerAuthCallbacks{
			KeyboardInteractiveCallback: keyboardInteractiveCallback,
		},
	}
}
