/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

export enum InstallParamEnrollMode {
  Script = 1,
}

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
  /**
   * install sets the join method when installing on
   * discovered EC2 nodes
   */
  install?: {
    /**
     * enrollMode indicates the mode used to enroll the node into Teleport.
     */
    enrollMode: InstallParamEnrollMode;
    /**
     * installTeleport disables agentless discovery
     */
    installTeleport: boolean;
    /**
     * joinToken is the token to use when joining the cluster
     */
    joinToken: string;
  };
  /**
   * ssm provides options to use when sending a document command to
   * an EC2 node
   */
  ssm?: { documentName: string };
};

type Labels = Record<string, string[]>;
