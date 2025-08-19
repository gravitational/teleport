/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/gravitational/trace"

	awsutils "github.com/gravitational/teleport/api/utils/aws"
)

// AWSClientConfig contains the required fields to set up an AWS SDK Clients.
type AWSClientConfig struct {
	// Credentials contains the AWS credentials to use.
	Credentials Credentials

	// Region where the API call should be made.
	Region string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *AWSClientConfig) checkAndSetDefaults() error {
	if req.Credentials.AccessKeyID == "" ||
		req.Credentials.SecretAccessKey == "" ||
		req.Credentials.SessionToken == "" {

		return trace.BadParameter("credentials are missing")
	}

	if req.Region != "" {
		if err := awsutils.IsValidRegion(req.Region); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// newAWSConfig creates a new [aws.Config] using the [AWSClientConfig] fields.
func newAWSConfig(ctx context.Context, req *AWSClientConfig) (aws.Config, error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	awsConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(req.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				req.Credentials.AccessKeyID,
				req.Credentials.SecretAccessKey,
				req.Credentials.SessionToken,
			),
		),
	)
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	return awsConfig, nil
}
