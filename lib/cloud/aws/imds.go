/*
Copyright 2022 Gravitational, Inc.

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
package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// InstanceMetadataClient is a wrapper for an imds.Client.
type InstanceMetadataClient struct {
	c         *imds.Client
	stsClient STSV2Client
}

// InstanceMetadataClientOption allows setting options as functional arguments to an InstanceMetadataClient.
type InstanceMetadataClientOption func(client *InstanceMetadataClient) error

// WithIMDSClient adds a custom internal imds.Client to an InstanceMetadataClient.
func WithIMDSClient(client *imds.Client) InstanceMetadataClientOption {
	return func(clt *InstanceMetadataClient) error {
		clt.c = client
		return nil
	}
}

// WithSTSClient adds a custom internal sts.Client to an InstanceMetadataClient.
func WithSTSClient(stsClient STSV2Client) InstanceMetadataClientOption {
	return func(clt *InstanceMetadataClient) error {
		clt.stsClient = stsClient
		return nil
	}
}

func currentRegion(ctx context.Context, cfg aws.Config, clt *imds.Client) (string, error) {
	// Use the region loaded on default config.
	if cfg.Region != "" {
		return cfg.Region, nil
	}

	// Fetch the region using the IMDS client.
	awsRegion, err := clt.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return awsRegion.Region, nil
}

// NewInstanceMetadataClient creates a new instance metadata client.
//
// GetInstanceMetadata may return an empty InstanceMetadata if the STSV2Client is not available.
// NewInstanceMetadataClient tries to create one, but it might fail when the AWS Region is not defined.
// A STSV2Client can be provided using the WithSTSClient function.
func NewInstanceMetadataClient(ctx context.Context, opts ...InstanceMetadataClientOption) (*InstanceMetadataClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := &InstanceMetadataClient{
		c: imds.NewFromConfig(cfg),
	}

	for _, opt := range opts {
		if err := opt(clt); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if clt.stsClient == nil {
		cfg.Region, err = currentRegion(ctx, cfg, clt.c)
		if err != nil {
			log.Debug("InstanceMetadata is not available because the AWS region is unknown")
		} else {
			clt.stsClient = sts.NewFromConfig(cfg)
		}
	}

	return clt, nil
}

// GetType gets the cloud instance type.
func (client *InstanceMetadataClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeEC2
}

// EC2 resource ID is i-{8 or 17 hex digits}, see https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/resource-ids.html
var ec2ResourceIDRE = regexp.MustCompile("^i-[0-9a-f]{8}([0-9a-f]{9})?$")

// IsAvailable checks if instance metadata is available.
func (client *InstanceMetadataClient) IsAvailable(ctx context.Context) bool {
	// try to retrieve the instance id of our EC2 instance
	id, err := client.getMetadata(ctx, "instance-id")
	return err == nil && ec2ResourceIDRE.MatchString(id)
}

// getMetadata gets the raw metadata from a specified path.
func (client *InstanceMetadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	output, err := client.c.GetMetadata(ctx, &imds.GetMetadataInput{Path: path})
	if err != nil {
		return "", trace.Wrap(parseMetadataClientError(err))
	}
	defer output.Content.Close()
	body, err := utils.ReadAtMost(output.Content, teleport.MaxHTTPResponseSize)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(body), nil
}

// getTagKeys gets all of the EC2 tag keys.
func (client *InstanceMetadataClient) getTagKeys(ctx context.Context) ([]string, error) {
	body, err := client.getMetadata(ctx, "tags/instance")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return strings.Split(body, "\n"), nil
}

// getTagValue gets the value for a specified tag key.
func (client *InstanceMetadataClient) getTagValue(ctx context.Context, key string) (string, error) {
	body, err := client.getMetadata(ctx, fmt.Sprintf("tags/instance/%s", key))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return body, nil
}

// GetTags gets all of the EC2 instance's tags.
func (client *InstanceMetadataClient) GetTags(ctx context.Context) (map[string]string, error) {
	keys, err := client.getTagKeys(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tags := make(map[string]string, len(keys))
	for _, key := range keys {
		value, err := client.getTagValue(ctx, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tags[key] = value
	}
	return tags, nil
}

// GetHostname gets the hostname set by EC2 that Teleport
// should use, if any.
func (client *InstanceMetadataClient) GetHostname(ctx context.Context) (string, error) {
	value, err := client.getTagValue(ctx, types.CloudHostnameTag)
	return value, trace.Wrap(err)
}

// GetRegion gets the EC2 instance's region.
func (client *InstanceMetadataClient) GetRegion(ctx context.Context) (string, error) {
	getRegionOutput, err := client.c.GetRegion(ctx, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return getRegionOutput.Region, nil
}

// GetID gets the EC2 instance's ID.
func (client *InstanceMetadataClient) GetID(ctx context.Context) (string, error) {
	id, err := client.getMetadata(ctx, "instance-id")
	if err != nil {
		return "", trace.Wrap(err)
	}

	if !ec2ResourceIDRE.MatchString(id) {
		return "", trace.NotFound("instance-id not available")
	}

	return id, nil
}

// GetInstanceMetadata gets the EC2 instance's identity.
func (client *InstanceMetadataClient) GetInstanceMetadata(ctx context.Context) (*types.InstanceMetadata, error) {
	if client.stsClient == nil {
		return &types.InstanceMetadata{
			AWSIdentity: &types.AWSInstanceIdentity{},
		}, nil
	}

	awsIdentity, err := GetIdentityWithClientV2(ctx, client.stsClient)
	if trace.IsNotFound(err) {
		return &types.InstanceMetadata{
			AWSIdentity: &types.AWSInstanceIdentity{},
		}, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.InstanceMetadata{
		AWSIdentity: &types.AWSInstanceIdentity{
			AccountID:    awsIdentity.GetAccountID(),
			ARN:          awsIdentity.String(),
			ResourceType: awsIdentity.GetType(),
			ResourceName: awsIdentity.GetName(),
		},
	}, nil
}
