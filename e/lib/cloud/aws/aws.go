// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"
)

// BuildAWSConfig is a helper function allowing to build AWS config with the given region, role ARN and role tags.
func BuildAWSConfig(ctx context.Context, region, roleARN string, roleTags map[string]string) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, trace.Wrap(err)
	}

	awsConfig.Credentials = aws.NewCredentialsCache(
		stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsConfig), roleARN, func(options *stscreds.AssumeRoleOptions) {
			var tags []ststypes.Tag
			for k, v := range roleTags {
				tags = append(tags, ststypes.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				})
			}
			options.Tags = tags
		}),
	)
	return awsConfig, nil
}
