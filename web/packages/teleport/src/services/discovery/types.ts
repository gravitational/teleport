/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Regions } from '../integrations';

// DiscoveryConfig describes DiscoveryConfig fields.
// Used for auto discovery service.
export type DiscoveryConfig = {
  // name is the DiscoveryConfig name.
  name: string;
  // discoveryGroup is the Group of the DiscoveryConfig.
  discoveryGroup: string;
  // aws is a list of matchers for AWS resources.
  aws: AwsMatcher[];
};

type AwsMatcherTypes = 'rds' | 'eks' | 'ec2';

// AWSMatcher matches AWS EC2 instances, AWS EKS clusters and AWS Databases
export type AwsMatcher = {
  // types are AWS types to match, "ec2", "eks", "rds", "redshift", "elasticache",
  // or "memorydb".
  types: AwsMatcherTypes[];
  // regions are AWS regions to query for resources.
  regions: Regions[];
  // tags are AWS resource tags to match.
  tags: Labels;
  // integration is the integration name used to generate credentials to interact with AWS APIs.
  // Environment credentials will not be used when this value is set.
  integration: string;
  // kubeAppDiscovery specifies if Kubernetes App Discovery should be enabled for a discovered cluster.
  kubeAppDiscovery?: boolean;
};

type Labels = Record<string, string[]>;
