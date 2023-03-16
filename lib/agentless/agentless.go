/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package agentless provides functions to allow connecting to registered
// OpenSSH (agentless) nodes.
package agentless

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/utils"
)

// CertGenerator generates cetificates from a certificate request.
type CertGenerator interface {
	GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error)
}

// SignerFromSSHCertificate returns a function that attempts to
// create a [ssh.Signer] for the Identity in the provided [ssh.Certificate]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
func SignerFromSSHCertificate(certificate *ssh.Certificate, generator CertGenerator) func(context.Context) (ssh.Signer, error) {
	return func(ctx context.Context) (ssh.Signer, error) {
		validBefore := time.Unix(int64(certificate.ValidBefore), 0)
		ttl := time.Until(validBefore)

		clusterName := certificate.Permissions.Extensions[utils.CertExtensionAuthority]
		user := certificate.Permissions.Extensions[utils.CertTeleportUser]

		signer, err := createAuthSigner(ctx, user, clusterName, ttl, generator)
		return signer, trace.Wrap(err)
	}
}

// SignerFromAuthzContext returns a function that attempts to
// create a [ssh.Signer] for the Identity in the provided [ssh.Certificate]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
func SignerFromAuthzContext(authzCtx *authz.Context, generator CertGenerator) func(context.Context) (ssh.Signer, error) {
	return func(ctx context.Context) (ssh.Signer, error) {
		identity := authzCtx.Identity.GetIdentity()
		ttl := time.Until(identity.Expires)

		signer, err := createAuthSigner(ctx, authzCtx.User.GetName(), identity.TeleportCluster, ttl, generator)
		return signer, trace.Wrap(err)
	}
}

// createAuthSigner creates a [ssh.Signer] that is signed with
// OpenSSH CA and can be used to authenticate to agentless nodes.
func createAuthSigner(ctx context.Context, username, clusterName string, ttl time.Duration, generator CertGenerator) (ssh.Signer, error) {
	// generate a new key pair
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sign new public key with OpenSSH CA
	reply, err := generator.GenerateOpenSSHCert(ctx, &proto.OpenSSHCertRequest{
		Username:  username,
		PublicKey: priv.MarshalSSHPublicKey(),
		TTL:       proto.Duration(ttl),
		Cluster:   clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// parse returned certificate bytes and create a signer with it
	cert, err := sshutils.ParseCertificate(reply.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privSigner, err := ssh.NewSignerFromSigner(priv.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewCertSigner(cert, privSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return signer, nil
}
