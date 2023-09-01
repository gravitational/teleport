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

	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth/native"
)

// EICESendSSHPublicKeyClient describes the required methods to send an SSH Public Key to
// an EC2 Instance.
// This is is remains for 60 seconds and is removed afterwards.
type EICESendSSHPublicKeyClient interface {
	// SendSSHPublicKey pushes an SSH public key to the specified EC2 instance for use by the specified
	// user. The key remains for 60 seconds. For more information, see Connect to your
	// Linux instance using EC2 Instance Connect (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Connect-using-EC2-Instance-Connect.html)
	// in the Amazon EC2 User Guide.
	SendSSHPublicKey(ctx context.Context, params *ec2instanceconnect.SendSSHPublicKeyInput, optFns ...func(*ec2instanceconnect.Options)) (*ec2instanceconnect.SendSSHPublicKeyOutput, error)
}

type defaultEICESendSSHPublicKeyClient struct {
	*ec2instanceconnect.Client
}

// NewEICESendSSHPublicKeyClient creates a EICESendSSHPublicKeyClient using AWSClientRequest.
func NewEICESendSSHPublicKeyClient(ctx context.Context, clientReq *AWSClientRequest) (EICESendSSHPublicKeyClient, error) {
	ec2instanceconnectClient, err := newEC2InstanceConnectClient(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultEICESendSSHPublicKeyClient{
		Client: ec2instanceconnectClient,
	}, nil
}

// SendSSHPublicKeyToEC2Request contains the required fields to request the upload of an SSH Public Key.
type SendSSHPublicKeyToEC2Request struct {
	// InstanceID is the EC2 Instance's ID.
	InstanceID string

	// EC2SSHLoginUser is the OS user to use when the user wants SSH access.
	EC2SSHLoginUser string
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *SendSSHPublicKeyToEC2Request) CheckAndSetDefaults() error {
	if r.InstanceID == "" {
		return trace.BadParameter("instance id is required")
	}

	if r.EC2SSHLoginUser == "" {
		return trace.BadParameter("ec2 ssh login user is required")
	}

	return nil
}

// SendSSHPublicKeyToEC2 sends an SSH Public Key to a target EC2 Instance.
// This key will be removed by AWS after 60 seconds and can only be used to authenticate the EC2SSHLoginUser.
// An [ssh.Signer] is then returned and can be used to access the host.
func SendSSHPublicKeyToEC2(ctx context.Context, clt EICESendSSHPublicKeyClient, req SendSSHPublicKeyToEC2Request) (ssh.Signer, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	sshSigner, err := sendSSHPublicKey(ctx, clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sshSigner, nil
}

// sendSSHPublicKey creates a new Private Key and uploads the Public to the ec2 instance.
// This key will be removed by AWS after 60 seconds and can only be used to authenticate the EC2SSHLoginUser.
// More information: https://docs.aws.amazon.com/ec2-instance-connect/latest/APIReference/API_SendSSHPublicKey.html
func sendSSHPublicKey(ctx context.Context, clt EICESendSSHPublicKeyClient, req SendSSHPublicKeyToEC2Request) (ssh.Signer, error) {
	pubKey, privKey, err := native.GenerateEICEKey()
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
