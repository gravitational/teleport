/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"fmt"
)

// GetRedshiftPolicy returns example AWS IAM policy for Redshift.
func GetRedshiftPolicy() string {
	return redshiftPolicy
}

// GetRDSPolicy returns example AWS IAM policy for RDS in the specified region.
func GetRDSPolicy(region string) string {
	return fmt.Sprintf(rdsPolicy, region)
}

// redshiftPolicy is the example Redshift policy template.
var redshiftPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "redshift:GetClusterCredentials",
      "Resource": [
        "arn:aws:redshift:*:AWS_ACCOUNT_ID:dbuser:*/*",
        "arn:aws:redshift:*:AWS_ACCOUNT_ID:dbname:*/*",
        "arn:aws:redshift:*:AWS_ACCOUNT_ID:dbgroup:*/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": "redshift:DescribeClusters",
      "Resource": "*"
    }
  ]
}`

// rdsPolicy is the example RDS policy template.
var rdsPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "rds-db:connect",
      "Resource": "arn:aws:rds-db:%v:AWS_ACCOUNT_ID:dbuser:AWS_RESOURCE_ID/*"
    }
  ]
}`
