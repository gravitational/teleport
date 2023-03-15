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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
)

// SiteClientGetter returns an auth client to a given cluster.
type SiteClientGetter interface {
	GetSiteClient(ctx context.Context, clusterName string) (auth.ClientI, error)
}

// CreateAuthSigner attempts to create a [ssh.Signer] that is signed with
// OpenSSH CA and can be used to authenticate to agentless nodes.
func CreateAuthSigner(ctx context.Context, username, clusterName string, ttl time.Duration, clientGetter SiteClientGetter) (ssh.Signer, error) {
	// generate a new key pair
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sign new public key with OpenSSH CA
	client, err := clientGetter.GetSiteClient(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reply, err := client.GenerateOpenSSHCert(ctx, &proto.OpenSSHCertRequest{
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
