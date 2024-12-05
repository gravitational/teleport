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
	"crypto/rand"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

// GenerateGitHubUserCert signs a SSH certificate for GitHub integration.
func (s *Service) GenerateGitHubUserCert(ctx context.Context, in *integrationpb.GenerateGitHubUserCertRequest) (*integrationpb.GenerateGitHubUserCertResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("GenerateGitHubUserCert is only available to proxy services")
	}

	cert, err := s.prepareGitHubCert(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caSigner, err := s.getGitHubSigner(ctx, in.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cert.SignCert(rand.Reader, caSigner); err != nil {
		return nil, trace.Wrap(err)
	}
	return &integrationpb.GenerateGitHubUserCertResponse{
		AuthorizedKey: ssh.MarshalAuthorizedKey(cert),
	}, nil
}

func (s *Service) prepareGitHubCert(in *integrationpb.GenerateGitHubUserCertRequest) (*ssh.Certificate, error) {
	if in.UserId == "" {
		return nil, trace.BadParameter("missing UserId for GenerateGitHubUserCert")
	}
	if in.KeyId == "" {
		return nil, trace.BadParameter("missing KeyId for GenerateGitHubUserCert")
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(in.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Sign with user ID set in id@github.com extension.
	// https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-git-access-to-your-organizations-repositories/about-ssh-certificate-authorities
	now := s.clock.Now()
	cert := &ssh.Certificate{
		Key:         key,
		CertType:    ssh.UserCert,
		KeyId:       in.KeyId,
		ValidAfter:  uint64(now.Add(-time.Minute).Unix()),
		ValidBefore: uint64(now.Add(in.Ttl.AsDuration()).Unix()),
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"id@github.com": in.UserId,
			},
		},
	}
	return cert, nil
}

func (s *Service) getGitHubSigner(ctx context.Context, integration string) (ssh.Signer, error) {
	ig, err := s.cache.GetIntegration(ctx, integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caKeySet, err := s.getGitHubCertAuthorities(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caSigner, err := s.keyStoreManager.GetSSHSignerFromKeySet(ctx, *caKeySet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return caSigner, nil
}
