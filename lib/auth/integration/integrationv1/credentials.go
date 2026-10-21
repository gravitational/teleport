/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integrationv1

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// ExportIntegrationCertAuthorities exports cert authorities for an integration.
func (s *Service) ExportIntegrationCertAuthorities(ctx context.Context, in *integrationpb.ExportIntegrationCertAuthoritiesRequest) (*integrationpb.ExportIntegrationCertAuthoritiesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindIntegration, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	ig, err := s.cache.GetIntegration(ctx, in.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caKeySet, err := credentials.GetIntegrationCertAuthorities(ctx, ig, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Currently only public keys are exported.
	caKeySetWithoutSecrets := caKeySet.WithoutSecrets()
	return integrationpb.ExportIntegrationCertAuthoritiesResponse_builder{
		CertAuthorities: &caKeySetWithoutSecrets,
	}.Build(), nil
}

// buildUpdatedOAuthSecret builds the OAuth client secret to update for the provided integration.
func (s *Service) buildUpdatedOAuthSecret(ctx context.Context, newIg, oldIg types.Integration, purpose string) (*types.PluginIdSecretCredential, error) {
	oauthIdSecret := newIg.GetCredentials().GetIdSecret()
	switch {
	case oauthIdSecret == nil || (oauthIdSecret.Id == "" && oauthIdSecret.Secret == ""):
		return nil, trace.BadParameter("integration requires OAuth credentials")
	case oauthIdSecret.Id != "" && oauthIdSecret.Secret == "":
		return nil, trace.BadParameter("missing OAuth secret for OAuth credentials")
	case oauthIdSecret.Id == "" && oauthIdSecret.Secret != "":
		// Special case where only secret is getting updated.
		oldIdSecret, err := s.getStaticCredentialsWithPurpose(ctx, oldIg, purpose)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		oldId, _ := oldIdSecret.GetOAuthClientSecret()
		oauthIdSecret.Id = oldId
		s.logger.DebugContext(ctx, "Updating integration with existing OAuth client ID", "purpose", purpose, "integration", newIg.GetName())
	}

	return oauthIdSecret, nil
}

// buildOAuthCredentials builds OAuth static credentials for the given purpose.
func buildOAuthCredentials(idSecret *types.PluginIdSecretCredential, purpose string) (*types.PluginStaticCredentialsV1, error) {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
				Labels: map[string]string{
					credentials.LabelStaticCredentialsPurpose: purpose,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     idSecret.Id,
					ClientSecret: idSecret.Secret,
				},
			},
		},
	}, nil
}

func (s *Service) newGitHubSSHCA(ctx context.Context) (*types.PluginStaticCredentialsV1, error) {
	ca, err := s.keyStoreManager.NewSSHKeyPair(ctx, cryptosuites.GitHubProxyCASSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
				Labels: map[string]string{
					credentials.LabelStaticCredentialsPurpose: credentials.PurposeGitHubSSHCA,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_SSHCertAuthorities{
				SSHCertAuthorities: &types.PluginStaticCredentialsSSHCertAuthorities{
					CertAuthorities: []*types.SSHKeyPair{ca},
				},
			},
		},
	}, nil
}

// createGitHubCredentials builds and persists credentials for the provided GitHub integration.
func (s *Service) createGitHubCredentials(ctx context.Context, ig types.Integration) error {
	var creds []types.PluginStaticCredentials

	if ig.GetCredentials() == nil || ig.GetCredentials().GetIdSecret() == nil {
		return trace.BadParameter("GitHub integration requires OAuth ID and secret for credentials")
	}
	if oauthCred, err := buildOAuthCredentials(ig.GetCredentials().GetIdSecret(), credentials.PurposeGitHubOAuth); err != nil {
		return trace.Wrap(err)
	} else {
		creds = append(creds, oauthCred)
	}

	// TODO(greedy52) support per auth CA like HSM.
	if caCred, err := s.newGitHubSSHCA(ctx); err != nil {
		return trace.Wrap(err)
	} else {
		creds = append(creds, caCred)
	}

	return trace.Wrap(s.createStaticCredentials(ctx, ig, creds...))
}

func (s *Service) createStaticCredentials(ctx context.Context, ig types.Integration, creds ...types.PluginStaticCredentials) error {
	ref := credentials.NewRef()

	for _, cred := range creds {
		s.logger.DebugContext(ctx, "Creating static credentials", "integration", ig.GetName(), "labels", cred.GetStaticLabels())
		credentials.CopyRefLabels(cred, ref)
		if err := s.backend.CreatePluginStaticCredentials(ctx, cred); err != nil {
			return trace.Wrap(err)
		}
	}

	ig.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: ref,
		},
	})
	return nil
}

func (s *Service) maybeUpdateStaticCredentials(ctx context.Context, newIg, oldIg types.Integration) error {
	// Preserve existing credentials.
	if newIg.GetCredentials() == nil {
		return trace.Wrap(newIg.SetCredentials(oldIg.GetCredentials()))
	}

	switch newIg.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		oauthIdSecret, err := s.buildUpdatedOAuthSecret(ctx, newIg, oldIg, credentials.PurposeGitHubOAuth)
		if err != nil {
			return trace.Wrap(err)
		}
		oauthCred, err := buildOAuthCredentials(oauthIdSecret, credentials.PurposeGitHubOAuth)
		if err != nil {
			return trace.Wrap(err)
		}
		// Copy ref from old integration and overwrite the OAuth settings.
		if err := newIg.SetCredentials(oldIg.GetCredentials()); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.updateStaticCredentials(ctx, newIg, oauthCred))
	}
	return nil
}

func (s *Service) updateStaticCredentials(ctx context.Context, ig types.Integration, creds ...types.PluginStaticCredentials) error {
	if ig.GetCredentials() == nil || ig.GetCredentials().GetStaticCredentialsRef() == nil {
		return trace.BadParameter("missing credentials ref")
	}

	ref := ig.GetCredentials().GetStaticCredentialsRef()
	for _, cred := range creds {
		s.logger.DebugContext(ctx, "Updating static credentials", "integration", ig.GetName(), "labels", cred.GetStaticLabels())

		// Use same labels to find existing credentials.
		credentials.CopyRefLabels(cred, ref)
		oldCreds, err := s.backend.GetPluginStaticCredentialsByLabels(ctx, cred.GetStaticLabels())
		if err != nil {
			return trace.Wrap(err)
		}
		if len(oldCreds) != 1 {
			return trace.CompareFailed("expecting one credential but got %v", len(oldCreds))
		}

		cred.SetName(oldCreds[0].GetName())
		cred.SetRevision(oldCreds[0].GetRevision())
		if _, err := s.backend.UpdatePluginStaticCredentials(ctx, cred); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Service) removeStaticCredentials(ctx context.Context, ig types.Integration) error {
	if ig.GetCredentials() == nil {
		return nil
	}

	ref := ig.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return trace.NotFound("missing static credentials ref")
	}

	staticCreds, err := s.backend.GetPluginStaticCredentialsByLabels(ctx, ref.Labels)
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	for _, cred := range staticCreds {
		s.logger.DebugContext(ctx, "Removing static credentials", "integration", ig.GetName(), "labels", cred.GetStaticLabels())
		if err := s.backend.DeletePluginStaticCredentials(ctx, cred.GetName()); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (s *Service) getStaticCredentialsWithPurpose(ctx context.Context, ig types.Integration, purpose string) (types.PluginStaticCredentials, error) {
	if ig.GetCredentials() == nil {
		return nil, trace.BadParameter("missing credentials")
	}

	return credentials.GetByPurpose(ctx, ig.GetCredentials().GetStaticCredentialsRef(), purpose, s.backend)
}

// appendCreateOAuthProxyCredentialActions builds the OAuth credential for the integration,
// and appends a conditional action to atomically put it.
func (s *Service) appendCreateOAuthProxyCredentialActions(actions []backend.ConditionalAction, ig types.Integration) ([]backend.ConditionalAction, error) {
	if ig.GetCredentials() == nil || ig.GetCredentials().GetIdSecret() == nil {
		return nil, trace.BadParameter("OAuth Proxy integration requires OAuth client ID and client secret for credentials")
	}

	cred, err := buildOAuthCredentials(ig.GetCredentials().GetIdSecret(), credentials.PurposeOAuthProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred.Metadata.Expires = integrationExpiry(ig)

	ref := credentials.NewRef()
	credentials.CopyRefLabels(cred, ref)

	actions, err = s.backend.AppendPutPluginStaticCredentialsActions(actions, cred, backend.NotExists())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ig.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: ref,
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return actions, nil
}

// appendUpdateOAuthProxyCredentialActions builds the OAuth credential for the integration,
// and appends a conditional action to atomically update it.
func (s *Service) appendUpdateOAuthProxyCredentialActions(ctx context.Context, actions []backend.ConditionalAction, newIg types.Integration, oldIg types.Integration) ([]backend.ConditionalAction, error) {
	if newIg.GetCredentials() == nil || newIg.GetCredentials().GetIdSecret() == nil {
		if err := oldIg.GetOAuthProxyIntegrationSpec().CanUpdateWithoutSecret(newIg.GetOAuthProxyIntegrationSpec()); err != nil {
			return nil, trace.Wrap(err)
		}

		// Preserve existing credentials.
		if err := newIg.SetCredentials(oldIg.GetCredentials()); err != nil {
			return nil, trace.Wrap(err)
		}

		return s.appendOAuthProxyCredentialExpiryActions(ctx, actions, newIg, oldIg)
	}

	if oldIg.GetCredentials() == nil {
		return nil, trace.BadParameter("missing existing credentials")
	}

	oauthIdSecret, err := s.buildUpdatedOAuthSecret(ctx, newIg, oldIg, credentials.PurposeOAuthProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cred, err := buildOAuthCredentials(oauthIdSecret, credentials.PurposeOAuthProxyClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Copy ref from old integration and overwrite the OAuth settings.
	if err := newIg.SetCredentials(oldIg.GetCredentials()); err != nil {
		return nil, trace.Wrap(err)
	}

	ref := oldIg.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return nil, trace.BadParameter("missing existing credentials ref")
	}
	credentials.CopyRefLabels(cred, ref)

	oldCreds, err := s.backend.GetPluginStaticCredentialsByLabels(ctx, cred.GetStaticLabels())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(oldCreds) != 1 {
		return nil, trace.CompareFailed("expecting one credential but got %v", len(oldCreds))
	}

	cred.SetName(oldCreds[0].GetName())
	cred.SetRevision(oldCreds[0].GetRevision())
	cred.Metadata.Expires = integrationExpiry(newIg)

	actions, err = s.backend.AppendPutPluginStaticCredentialsActions(actions, cred, backend.Revision(oldCreds[0].GetRevision()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return actions, nil
}

func (s *Service) appendOAuthProxyCredentialExpiryActions(ctx context.Context, actions []backend.ConditionalAction, newIg, oldIg types.Integration) ([]backend.ConditionalAction, error) {
	ref := oldIg.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return nil, trace.BadParameter("missing existing credentials ref")
	}

	cred, err := credentials.GetByPurpose(ctx, ref, credentials.PurposeOAuthProxyClient, s.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credV1, ok := cred.(*types.PluginStaticCredentialsV1)
	if !ok {
		return nil, trace.BadParameter("expected *types.PluginStaticCredentialsV1, got %T", cred)
	}

	credV1.Metadata.Expires = integrationExpiry(newIg)

	actions, err = s.backend.AppendPutPluginStaticCredentialsActions(actions, credV1, backend.Revision(credV1.GetRevision()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return actions, nil
}

// integrationExpiry copies an expiry from an integration to use on credentials whose lifetime must match the integration's.
// It needs to be copied rather than use SetExpiry otherwise an absent expiry on the integration results in a zero value expiry on the credential.
func integrationExpiry(ig types.Integration) *time.Time {
	expires := ig.GetMetadata().Expires
	if expires == nil {
		return nil
	}

	cloned := *expires
	return &cloned
}
