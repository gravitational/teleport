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

package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func (a *Server) getSSHCASigner(ctx context.Context, domainName string, caType types.CertAuthType) (ssh.Signer, error) {
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: domainName,
	}, true /*loadKeys*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := a.GetKeyStore().GetSSHSigner(ctx, ca)
	return signer, trace.Wrap(err)
}

// TODO
func (a *Server) SignGitHubUserCert(ctx context.Context, req *proto.SignGitHubUserCertRequest) (*proto.SignGitHubUserCertResponse, error) {
	// TODO check modules?
	slog.DebugContext(ctx, "Signing GitHub cert.", "key_id", req.KeyID, "login", req.Login)

	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubCASigner, err := a.getSSHCASigner(ctx, domainName, types.GitHubCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	now := a.clock.Now()
	validAfter := now.Add(-time.Minute)

	newSSHCert := &ssh.Certificate{
		Key:         publicKey,
		CertType:    ssh.UserCert,
		KeyId:       req.KeyID,
		ValidAfter:  uint64(validAfter.Unix()),
		ValidBefore: uint64(req.Expires.Unix()),
	}
	newSSHCert.Extensions = map[string]string{
		"login@github.com": req.Login,
	}
	if err := newSSHCert.SignCert(rand.Reader, githubCASigner); err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.SignGitHubUserCertResponse{
		AuthorizedKey: ssh.MarshalAuthorizedKey(newSSHCert),
	}, nil
}

func (a *Server) GenerateGitServerCert(ctx context.Context, req *proto.GenerateGitServerCertRequest) (*proto.GenerateGitServerCertResponse, error) {
	// TODO check modules?
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostID := fmt.Sprintf("%s.teleport-git-app", req.AppName)
	slog.DebugContext(ctx, "Generating git server cert.", "id", hostID)

	hostCASigner, err := a.getSSHCASigner(ctx, domainName, types.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshCert, err := a.Authority.GenerateHostCert(services.HostCertParams{
		CASigner:      hostCASigner,
		PublicHostKey: req.PublicKey,
		HostID:        hostID,
		Role:          types.RoleApp,
		ClusterName:   clusterName.GetClusterName(),
		TTL:           req.TTL.Get(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.GenerateGitServerCertResponse{
		SshCertificate: sshCert,
	}, nil
}
