// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"net"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ec2v1 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
)

// defaultSSHPort is the default port for the OpenSSH Service.
const defaultSSHPort = "22"

// NewAWSNodeFromEC2Instance creates a Node resource from an EC2 Instance.
// It has a pre-populated spec which contains info that is not available in the ec2.Instance object.
func NewAWSNodeFromEC2Instance(instance ec2types.Instance, awsCloudMetadata *types.AWSInfo) (types.Server, error) {
	labels := libaws.TagsToLabels(instance.Tags)
	if labels == nil {
		labels = make(map[string]string)
	}
	libaws.AddMetadataLabels(labels, awsCloudMetadata.AccountID, awsCloudMetadata.Region)

	instanceID := aws.ToString(instance.InstanceId)
	labels[types.AWSInstanceIDLabel] = instanceID
	labels[types.AWSAccountIDLabel] = awsCloudMetadata.AccountID

	awsCloudMetadata.InstanceID = instanceID
	awsCloudMetadata.VPCID = aws.ToString(instance.VpcId)
	awsCloudMetadata.SubnetID = aws.ToString(instance.SubnetId)

	if aws.ToString(instance.PrivateIpAddress) == "" {
		return nil, trace.BadParameter("private ip address is required from ec2 instance")
	}
	// Address requires the Port.
	// We use the default port for the OpenSSH daemon.
	addr := net.JoinHostPort(aws.ToString(instance.PrivateIpAddress), defaultSSHPort)

	hostname := aws.ToString(instance.PrivateDnsName)
	if hostnameFromTag, ok := labels[types.CloudHostnameTag]; ok {
		hostname = hostnameFromTag
	}

	server, err := types.NewEICENode(
		types.ServerSpecV2{
			Hostname: hostname,
			Addr:     addr,
			CloudMetadata: &types.CloudMetadata{
				AWS: awsCloudMetadata,
			},
		},
		labels,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server, nil
}

// NewAWSNodeFromEC2v1Instance creates a Node resource from an EC2 Instance.
// It has a pre-populated spec which contains info that is not available in the ec2.Instance object.
// Uses AWS SDK Go V1
func NewAWSNodeFromEC2v1Instance(instance ec2v1.Instance, awsCloudMetadata *types.AWSInfo) (types.Server, error) {
	server, err := NewAWSNodeFromEC2Instance(ec2InstanceV1ToV2(instance), awsCloudMetadata)
	return server, trace.Wrap(err)
}

func ec2InstanceV1ToV2(instance ec2v1.Instance) ec2types.Instance {
	tags := make([]ec2types.Tag, 0, len(instance.Tags))
	for _, tag := range instance.Tags {
		tags = append(tags, ec2types.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	return ec2types.Instance{
		InstanceId:       instance.InstanceId,
		VpcId:            instance.VpcId,
		SubnetId:         instance.SubnetId,
		PrivateIpAddress: instance.PrivateIpAddress,
		PrivateDnsName:   instance.PrivateDnsName,
		Tags:             tags,
	}
}
