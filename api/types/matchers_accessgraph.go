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

package types

import (
	"github.com/gravitational/trace"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
)

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (a *AccessGraphSync) CheckAndSetDefaults() error {
	for _, matcher := range a.AWS {
		if err := matcher.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *AccessGraphAWSSync) CheckAndSetDefaults() error {
	if len(a.Regions) == 0 {
		return trace.BadParameter("discovery service requires at least one region")
	}

	for _, region := range a.Regions {
		if err := awsapiutils.IsValidRegion(region); err != nil {
			return trace.BadParameter("discovery service does not support region %q", region)
		}
	}

	if a.CloudTrailLogs != nil {
		if a.CloudTrailLogs.SQSQueue == "" {
			return trace.BadParameter("discovery service requires SQS queue for CloudTrail logs")
		}
		if a.CloudTrailLogs.Region == "" {
			return trace.BadParameter("discovery service requires Region for CloudTrail logs")
		}
	}

	return nil
}
