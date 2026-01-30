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
	"context"
	"os"
	"slices"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
	"github.com/gravitational/teleport/lib/sshca"
)

// KeyboardInteractiveAuth performs keyboard-interactive authentication based on the provided preconditions. If no
// preconditions are provided, it returns the input permissions as-is. If further authentication is required, it returns
// a PartialSuccessError containing the necessary SSH server auth callbacks that the SSH server can use to continue the
// authentication process.
func (h *AuthHandlers) KeyboardInteractiveAuth(
	ctx context.Context,
	preconds []*decisionpb.Precondition,
	id *sshca.Identity,
	perms *ssh.Permissions,
) (*ssh.Permissions, error) {
	if len(preconds) == 0 {
		return perms, nil
	}

	// If an unknown or unsupported precondition is provided, fail close to prevent potential authentication bypasses.
	if err := ensureSupportedPreconditions(preconds); err != nil {
		return nil, trace.Wrap(err)
	}

	// At this point we know that the client already completed public key authentication because this method should only
	// be called after successful public key authentication (bug otherwise). We don't know yet whether the client is a
	// legacy client that only supports public key authentication or a modern client that supports keyboard-interactive
	// authentication. Therefore, we will set up both callbacks and let the SSH server decide which one to invoke based
	// on what the client supports.

	// legacyPublicKeyCallback allows a legacy client to proceed with just public key authentication for backwards
	// compatibility, skipping keyboard-interactive authentication altogether. If MFA is required by the preconditions,
	// only per-session MFA certificates are allowed since they indicate that MFA was already performed (see RFD 0234).
	//
	// TODO(cthach): Remove in v20.0 and only set KeyboardInteractiveCallback.
	legacyPublicKeyCallback := func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
		if err := denyRegularSSHCertsIfMFARequired(preconds, id); err != nil {
			return nil, trace.Wrap(err)
		}

		return perms, nil
	}
	if os.Getenv("TELEPORT_UNSTABLE_FORCE_IN_BAND_MFA") == "yes" {
		legacyPublicKeyCallback = func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, trace.AccessDenied(`legacy public key authentication is forbidden (TELEPORT_UNSTABLE_FORCE_IN_BAND_MFA = "yes")`)
		}
	}

	// keyboardInteractiveCallback handles keyboard-interactive authentication for modern clients.
	keyboardInteractiveCallback := func(metadata ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
		var verifiers []srvssh.PromptVerifier

		for _, p := range preconds {
			switch p.GetKind() {
			case decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA:
				// TODO(cthach): Use the source cluster name that the client will do the MFA ceremony with.
				verifier, err := srvssh.NewMFAPromptVerifier(h.c.ValidatedMFAChallengeVerifier, id.ClusterName, id.Username, metadata.SessionID())
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
			PublicKeyCallback:           legacyPublicKeyCallback,
			KeyboardInteractiveCallback: keyboardInteractiveCallback,
		},
	}
}

func ensureSupportedPreconditions(preconds []*decisionpb.Precondition) error {
	for _, precond := range preconds {
		switch precond.GetKind() {
		case decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA:
			// OK

		default:
			return trace.BadParameter("unexpected precondition type %q found (this is a bug)", precond.GetKind())
		}
	}

	return nil
}

func denyRegularSSHCertsIfMFARequired(
	preconds []*decisionpb.Precondition,
	id *sshca.Identity,
) error {
	// Determine if MFA is required based on the provided preconditions.
	mfaRequired := slices.ContainsFunc(
		preconds,
		func(p *decisionpb.Precondition) bool {
			return p.GetKind() == decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA
		},
	)

	// A regular SSH certificate is one that does not have per-session MFA verification.
	isRegularSSHCert := id.MFAVerified == "" && !id.PrivateKeyPolicy.MFAVerified()

	if mfaRequired && isRegularSSHCert {
		return trace.AccessDenied("regular SSH certificates are forbidden when MFA is required and using legacy public key authentication")
	}

	return nil
}
