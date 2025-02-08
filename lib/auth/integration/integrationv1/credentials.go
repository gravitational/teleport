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

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
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
	ig, err := s.cache.GetIntegration(ctx, in.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Currently only public keys are exported.
	switch ig.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		caKeySet, err := s.getGitHubCertAuthorities(ctx, ig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caKeySetWithoutSecerts := caKeySet.WithoutSecrets()
		return &integrationpb.ExportIntegrationCertAuthoritiesResponse{CertAuthorities: &caKeySetWithoutSecerts}, nil
	default:
		return nil, trace.BadParameter("unsupported for integration subkind %v", ig.GetSubKind())
	}
}

func buildGitHubOAuthCredentials(idSecret *types.PluginIdSecretCredential) (*types.PluginStaticCredentialsV1, error) {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
				Labels: map[string]string{
					credentials.LabelStaticCredentialsPurpose: credentials.PurposeGitHubOAuth,
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

func (s *Service) createGitHubCredentials(ctx context.Context, ig types.Integration) error {
	var creds []types.PluginStaticCredentials

	if ig.GetCredentials() == nil || ig.GetCredentials().GetIdSecret() == nil {
		return trace.BadParameter("GitHub integration requires OAuth ID and secret for credentials")
	}
	if oauthCred, err := buildGitHubOAuthCredentials(ig.GetCredentials().GetIdSecret()); err != nil {
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

func (s *Service) maybeUpdateStaticCredentials(ctx context.Context, newIg types.Integration) error {
	oldIg, err := s.backend.GetIntegration(ctx, newIg.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	// Preserve existing credentials.
	if newIg.GetCredentials() == nil {
		return trace.Wrap(newIg.SetCredentials(oldIg.GetCredentials()))
	}

	switch newIg.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		oauthIdSecret := newIg.GetCredentials().GetIdSecret()
		switch {
		case oauthIdSecret == nil || (oauthIdSecret.Id == "" && oauthIdSecret.Secret == ""):
			return trace.BadParameter("GitHub integration requires OAuth credentials")
		case oauthIdSecret.Id != "" && oauthIdSecret.Secret == "":
			return trace.BadParameter("missing OAuth secret for OAuth credentials")
		case oauthIdSecret.Id == "" && oauthIdSecret.Secret != "":
			// Special case where only secret is getting updated.
			oldIdSecret, err := s.getStaticCredentialsWithPurpose(ctx, oldIg, credentials.PurposeGitHubOAuth)
			if err != nil {
				return trace.Wrap(err)
			}
			oldId, _ := oldIdSecret.GetOAuthClientSecret()
			oauthIdSecret.Id = oldId
			s.logger.DebugContext(ctx, "Updating integration with existing OAuth client ID", "integration", newIg.GetName())
		}

		oauthCred, err := buildGitHubOAuthCredentials(oauthIdSecret)
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

	return credentials.GetByPurpose(ctx, ig.GetCredentials().GetStaticCredentialsRef(), purpose, s.cache)
}

func (s *Service) getGitHubCertAuthorities(ctx context.Context, ig types.Integration) (*types.CAKeySet, error) {
	if ig.GetSubKind() != types.IntegrationSubKindGitHub {
		return nil, trace.BadParameter("integration is not a GitHub integration")
	}
	creds, err := s.getStaticCredentialsWithPurpose(ctx, ig, credentials.PurposeGitHubSSHCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cas := creds.GetSSHCertAuthorities()
	if len(cas) == 0 {
		return nil, trace.BadParameter("missing SSH cert authorities from plugin static credentials")
	}
	return &types.CAKeySet{
		SSH: cas,
	}, nil
}
