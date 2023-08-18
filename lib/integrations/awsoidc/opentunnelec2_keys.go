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

package awsoidc

import (
	"context"
	"crypto/ed25519"

	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/modules"
)

// generatePrivatePublicKey returns a public and its private key usable for creating an [ssh.Signer].
func generatePrivatePublicKey() (publicKey any, privateKey any, err error) {
	if modules.GetModules().IsBoringBinary() {
		privKey, err := native.GeneratePrivateKey()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return privKey.Public(), privKey, nil
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return pubKey, privKey, nil
}

// sendSSHPublicKey creates a new Private Key and uploads the Public to the ec2 instance.
// This key is only valid for 60 seconds and can only be used to authenticate the EC2SSHLoginUser.
// More information: https://docs.aws.amazon.com/ec2-instance-connect/latest/APIReference/API_SendSSHPublicKey.html
func sendSSHPublicKey(ctx context.Context, clt OpenTunnelEC2Client, req OpenTunnelEC2Request) (ssh.Signer, error) {
	pubKey, privKey, err := generatePrivatePublicKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubKeySSH := string(ssh.MarshalAuthorizedKey(publicKey))
	_, err = clt.SendSSHPublicKey(ctx,
		&ec2instanceconnect.SendSSHPublicKeyInput{
			InstanceId:     &req.InstanceID,
			InstanceOSUser: &req.EC2SSHLoginUser,
			SSHPublicKey:   &pubKeySSH,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshSigner, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sshSigner, nil
}
