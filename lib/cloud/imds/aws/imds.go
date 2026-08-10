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

package aws

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// InstanceMetadataClient is a wrapper for an imds.Client.
type InstanceMetadataClient struct {
	c *imds.Client
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

// convertLoadConfigError converts common AWS config loading errors to trace errors.
func convertLoadConfigError(configErr error) error {
	var sharedConfigProfileNotExistError config.SharedConfigProfileNotExistError
	switch {
	case errors.As(configErr, &sharedConfigProfileNotExistError):
		return trace.NotFound("%s", configErr)
	}

	return configErr
}

// NewInstanceMetadataClient creates a new instance metadata client.
func NewInstanceMetadataClient(ctx context.Context, opts ...InstanceMetadataClientOption) (*InstanceMetadataClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(convertLoadConfigError(err))
	}

	clt := &InstanceMetadataClient{
		c: imds.NewFromConfig(cfg),
	}

	for _, opt := range opts {
		if err := opt(clt); err != nil {
			return nil, trace.Wrap(err)
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

// parseMetadataClientError converts a failed instance metadata service call to a trace error.
func parseMetadataClientError(err error) error {
	var httpError interface{ HTTPStatusCode() int }
	if errors.As(err, &httpError) {
		return trace.ReadError(httpError.HTTPStatusCode(), nil)
	}
	return trace.Wrap(err)
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

// GetLocalIPV4 gets the EC2 instance's local ipv4 address.
func (client *InstanceMetadataClient) GetLocalIPV4(ctx context.Context) (string, error) {
	ip, err := client.getMetadata(ctx, "local-ipv4")
	if err != nil {
		return "", trace.Wrap(err)
	}

	return ip, nil
}

// GetPublicIPV4 gets the EC2 instance's local ipv4 address.
func (client *InstanceMetadataClient) GetPublicIPV4(ctx context.Context) (string, error) {
	ip, err := client.getMetadata(ctx, "public-ipv4")
	if err != nil {
		return "", trace.Wrap(err)
	}

	return ip, nil
}

func (client *InstanceMetadataClient) GetAccountID(ctx context.Context) (string, error) {
	idOut, err := client.c.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return idOut.AccountID, nil
}
