/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package awsoidc

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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

	// PublicKey is the SSH public key to send.
	PublicKey ssh.PublicKey
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *SendSSHPublicKeyToEC2Request) CheckAndSetDefaults() error {
	if r.InstanceID == "" {
		return trace.BadParameter("instance id is required")
	}

	if r.EC2SSHLoginUser == "" {
		return trace.BadParameter("ec2 ssh login user is required")
	}

	if r.PublicKey == nil {
		return trace.BadParameter("SSH public key is required")
	}

	return nil
}

// SendSSHPublicKeyToEC2 sends an SSH Public Key to a target EC2 Instance.
// This key will be removed by AWS after 60 seconds and can only be used to authenticate the EC2SSHLoginUser.
// An [ssh.Signer] is then returned and can be used to access the host.
func SendSSHPublicKeyToEC2(ctx context.Context, clt EICESendSSHPublicKeyClient, req SendSSHPublicKeyToEC2Request) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := sendSSHPublicKey(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// sendSSHPublicKey creates a new Private Key and uploads the Public to the ec2 instance.
// This key will be removed by AWS after 60 seconds and can only be used to authenticate the EC2SSHLoginUser.
// More information: https://docs.aws.amazon.com/ec2-instance-connect/latest/APIReference/API_SendSSHPublicKey.html
func sendSSHPublicKey(ctx context.Context, clt EICESendSSHPublicKeyClient, req SendSSHPublicKeyToEC2Request) error {
	pubKeySSH := string(ssh.MarshalAuthorizedKey(req.PublicKey))
	_, err := clt.SendSSHPublicKey(ctx,
		&ec2instanceconnect.SendSSHPublicKeyInput{
			InstanceId:     &req.InstanceID,
			InstanceOSUser: &req.EC2SSHLoginUser,
			SSHPublicKey:   &pubKeySSH,
		},
	)
	return trace.Wrap(err)
}
