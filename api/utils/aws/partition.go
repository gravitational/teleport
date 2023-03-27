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

// GetPartitionFromRegion get aws partition from region
// example, region "us-east-1" corresponds to partition "aws"
// region "cn-north-1" corresponds to partition "aws-cn"
func GetPartitionFromRegion(region string) string {
	var partition string
	switch {
	case IsCNRegion(region):
		partition = CNPartition
	case IsUSGovRegion(region):
		partition = USGovPartition
	default:
		partition = StandardPartition
	}
	return partition
}

const (
	// StandardPartition is the partition ID of the AWS Standard partition.
	StandardPartition = "aws"

	// CNPartition is the partition ID of the AWS China partition.
	CNPartition = "aws-cn"

	// USGovPartition is the partition ID of the AWS GovCloud partition.
	USGovPartition = "aws-us-gov"
)
