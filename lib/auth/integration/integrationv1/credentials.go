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
	"maps"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

const (
	// labelStaticCredentialsIntegration is the label used to store the
	// UUID ref in the static credentials.
	labelStaticCredentialsIntegration = types.TeleportInternalLabelPrefix + types.KindIntegration
	// labelStaticCredentialsPurpose is the label used to store the purpose of
	// the static credentials.
	labelStaticCredentialsPurpose = "purpose"

	// purposeGitHubSSHCA is the label value that indicates the static
	// credentials contains the GitHub SSH CA.
	purposeGitHubSSHCA = "github-sshca"
	// purposeGitHubOAuth is the label value that indicates the static
	// credentials contains the GitHub OAuth ID and secret.
	purposeGitHubOAuth = "github-oauth"
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

func newStaticCredentialsRef(uuid string) *types.PluginStaticCredentialsRef {
	return &types.PluginStaticCredentialsRef{
		Labels: map[string]string{
			labelStaticCredentialsIntegration: uuid,
		},
	}
}

func copyRefLabels(cred types.PluginStaticCredentials, ref *types.PluginStaticCredentialsRef) {
	labels := cred.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, ref.Labels)

	cred.SetStaticLabels(labels)
}

func buildGitHubOAuthCredentials(ig types.Integration) (*types.PluginStaticCredentialsV1, error) {
	if ig.GetCredentials() == nil || ig.GetCredentials().GetIdSecret() == nil {
		return nil, trace.BadParameter("GitHub integration requires OAuth ID and secret for credentials")
	}

	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
				Labels: map[string]string{
					labelStaticCredentialsPurpose: purposeGitHubOAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     ig.GetCredentials().GetIdSecret().Id,
					ClientSecret: ig.GetCredentials().GetIdSecret().Secret,
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
					labelStaticCredentialsPurpose: purposeGitHubSSHCA,
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

	if oauthCred, err := buildGitHubOAuthCredentials(ig); err != nil {
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
	ref := newStaticCredentialsRef(uuid.NewString())

	for _, cred := range creds {
		s.logger.DebugContext(ctx, "Creating static credentials", "integration", ig.GetName(), "labels", cred.GetStaticLabels())
		copyRefLabels(cred, ref)
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
		newIg.SetCredentials(oldIg.GetCredentials())
		return nil
	}

	switch newIg.GetSubKind() {
	case types.IntegrationSubKindGitHub:
		if oauthCred, err := buildGitHubOAuthCredentials(newIg); err != nil {
			return trace.Wrap(err)
		} else {
			// Copy ref.
			newIg.SetCredentials(oldIg.GetCredentials())
			return trace.Wrap(s.updateStaticCredentials(ctx, newIg, oauthCred))
		}
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
		copyRefLabels(cred, ref)
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
	if ig.GetCredentials() == nil || ig.GetCredentials().GetStaticCredentialsRef() == nil {
		return nil, trace.BadParameter("missing credentials ref")
	}
	labels := ig.GetCredentials().GetStaticCredentialsRef().Labels
	if len(labels) == 0 {
		return nil, trace.BadParameter("missing labels from credentials ref")
	}
	labels[labelStaticCredentialsPurpose] = purpose

	// TODO(greedy52) use cache
	creds, err := s.backend.GetPluginStaticCredentialsByLabels(ctx, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch len(creds) {
	case 0:
		return nil, trace.NotFound("%v credentials not found", purpose)
	case 1:
		return creds[0], nil
	default:
		return nil, trace.CompareFailed("expecting one plugin static credentials but got %v", len(creds))
	}
}

func (s *Service) getGitHubCertAuthorities(ctx context.Context, ig types.Integration) (*types.CAKeySet, error) {
	if ig.GetSubKind() != types.IntegrationSubKindGitHub {
		return nil, trace.BadParameter("integration is not a GitHub integration")
	}
	creds, err := s.getStaticCredentialsWithPurpose(ctx, ig, purposeGitHubSSHCA)
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
