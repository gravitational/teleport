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
	"net"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// GenerateAndUploadKey creates ephemeral SSH credentials and uploads the public key to the target instance.
// The returned [ssh.Signer] must be used by clients to connect to the target instanace. The credentials
// will be removed by AWS after 60 seconds and can only be used to authenticate the provided [login].
// More information: https://docs.aws.amazon.com/ec2-instance-connect/latest/APIReference/API_SendSSHPublicKey.html
func GenerateAndUploadKey(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
	awsInfo := target.GetAWSInfo()
	if awsInfo == nil {
		return nil, trace.BadParameter("missing aws cloud metadata")
	}

	if awsInfo.InstanceID == "" {
		return nil, trace.BadParameter("instance id is required")
	}

	if login == "" {
		return nil, trace.BadParameter("ec2 ssh login user is required")
	}

	if integration == nil || integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, trace.BadParameter("integration does not have aws oidc spec fields %q", awsInfo.Integration)
	}

	ec2instanceconnectClient, err := newEC2InstanceConnectClient(ctx, &AWSClientRequest{
		Token:   token,
		RoleARN: integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:  awsInfo.Region,
	})
	if err != nil {
		return nil, trace.BadParameter("failed to create an aws client to send ssh public key:  %v", err)
	}

	sshKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(ap),
		cryptosuites.EC2InstanceConnect)
	if err != nil {
		return nil, trace.Wrap(err, "generating SSH key")
	}
	sshSigner, err := ssh.NewSignerFromSigner(sshKey)
	if err != nil {
		return nil, trace.Wrap(err, "creating SSH signer")
	}

	if _, err := ec2instanceconnectClient.SendSSHPublicKey(ctx, &ec2instanceconnect.SendSSHPublicKeyInput{
		InstanceId:     aws.String(awsInfo.InstanceID),
		InstanceOSUser: aws.String(login),
		SSHPublicKey:   aws.String(string(ssh.MarshalAuthorizedKey(sshSigner.PublicKey()))),
	}); err != nil {
		return nil, trace.BadParameter("send ssh public key failed for instance %s: %v", awsInfo.InstanceID, err)
	}

	// This is the SSH Signer that the client must use to connect to the EC2.
	// This signer is trusted because the public key was sent to the target EC2 host.
	return sshSigner, nil
}

// DialInstance opens a tunnel to the target host and returns a [net.Conn] that
// may be used to connect to the instance.
func DialInstance(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
	awsInfo := target.GetAWSInfo()
	if awsInfo == nil {
		return nil, trace.BadParameter("missing aws cloud metadata")
	}

	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil, trace.BadParameter("integration does not have aws oidc spec fields %q", awsInfo.Integration)
	}

	openTunnelClt, err := newOpenTunnelEC2Client(ctx, &AWSClientRequest{
		Token:   token,
		RoleARN: integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:  awsInfo.Region,
	})
	if err != nil {
		return nil, trace.BadParameter("failed to create the ec2 open tunnel client: %v", err)
	}

	openTunnelResp, err := openTunnelEC2(ctx, openTunnelClt, OpenTunnelEC2Request{
		Region:        awsInfo.Region,
		VPCID:         awsInfo.VPCID,
		EC2InstanceID: awsInfo.InstanceID,
		EC2Address:    target.GetAddr(),
	})
	if err != nil {
		return nil, trace.BadParameter("failed to open AWS EC2 Instance Connect Endpoint tunnel: %v", err)
	}

	// OpenTunnelResp has the tcp connection that should be used to access the EC2 instance directly.
	return openTunnelResp.Tunnel, nil
}
