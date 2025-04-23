/**
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

import { ResourceLabel } from '../agents';

export type JoinToken = {
  id: string;
  // safeName is the name represented by "*". If the name is longer than 16 chars,
  // the first 16 chars will be * and the rest of the token's chars will be visible
  // ex. ****************asdf1234
  safeName: string;
  // bot_name is present on tokens with Bot in their join roles
  bot_name?: string;
  isStatic: boolean;
  // the join method of the token
  method: string;
  // Roles are the roles granted to the token
  roles: string[];
  expiry: Date;
  expiryText?: string;
  // suggestedLabels are labels that the resource should add when adding
  // itself to the cluster
  suggestedLabels?: ResourceLabel[];
  // internalResourceId will be the unique id used to identify that
  // a resource was added using this join token.
  //
  // Extracted from suggestedLabels.
  internalResourceId?: string;
  // yaml content of the resource
  content: string;
  allow?: AWSRules[];
  gcp?: {
    allow: GCPRules[];
  };
  oracle?: {
    allow: OracleRules[];
  };
};

// JoinRole defines built-in system roles and are roles associated with
// a join token and will be granted to the user of the join token.
// Same hard-coded value as the backend:
// - 'App' is a role for an app proxy in the cluster
// - 'Db' is a role for a database proxy in the cluster
// - 'Kube' is a role for a kube service
// - 'Node' is a role for a node in the cluster
// - 'Bot' for MachineID (when set, "spec.bot_name" must be set in the token)
// - 'WindowsDesktop' is a role for a windows desktop service.
// - 'Discovery' is a role for a discovery service.
export type JoinRole =
  | 'App'
  | 'Node'
  | 'Db'
  | 'Kube'
  | 'Bot'
  | 'WindowsDesktop'
  | 'Discovery';

// JoinMethod is the method used for new nodes to join the cluster.
// Same hard-corded value as the backend.
// - 'token' is the default method, where nodes join the cluster by
//   presenting a secret token.
export type JoinMethod =
  | 'token'
  | 'ec2'
  | 'iam'
  | 'github'
  | 'azure'
  | 'gcp'
  | 'circleci'
  | 'gitlab'
  | 'kubernetes'
  | 'tpm'
  | 'oracle';

// JoinRule is a rule that a joining node must match in order to use the
// associated token.
export type JoinRule = {
  awsAccountId: string;
  // awsArn is used for the IAM join method.
  awsArn?: string;
  regions?: string[];
};

export type AWSRules = {
  aws_account: string; // naming kept consistent with backend spec
  aws_arn?: string;
};

export type GCPRules = {
  project_ids: string[];
  locations: string[];
  service_accounts: string[];
};

export type OracleRules = {
  tenancy: string;
  parent_compartments: string[];
  regions: string[];
};

export type JoinTokenRulesObject = AWSRules | GCPRules;

export type CreateJoinTokenRequest = {
  name: string;
  // roles is a list of join roles, since there can be more than
  // one role associated with a token.
  roles: JoinRole[];
  // bot_name only needs to be specified if "Bot" is in the selected roles.
  // otherwise, it is ignored
  bot_name?: string;
  join_method: JoinMethod;
  // rules is a list of allow rules associated with the join token
  // and the node using this token must match one of the rules.
  allow?: JoinTokenRulesObject[];
  gcp?: {
    allow: GCPRules[];
  };
  oracle?: {
    allow: OracleRules[];
  };
};

export type JoinTokenRequest = {
  // roles is a list of join roles, since there can be more than
  // one role associated with a token.
  roles?: JoinRole[];
  // rules is a list of allow rules associated with the join token
  // and the node using this token must match one of the rules.
  rules?: JoinRule[];
  // suggestedAgentMatcherLabels is a set of labels to be used by agents to match
  // on resources. When an agent uses this token, the agent should
  // monitor resources that match those labels. For databases, this
  // means adding the labels to `db_service.resources.labels`.
  suggestedAgentMatcherLabels?: ResourceLabel[];
  method?: JoinMethod;
  // content is the yaml content of the joinToken to be created
  content?: string;
  /**
   * User provided labels.
   * SuggestedLabels is a set of labels that resources should set when using this token to enroll
   * themselves in the cluster.
   * Currently, only node-join scripts create a configuration according to the suggestion.
   *
   * Only supported with V2 endpoint.
   */
  suggestedLabels?: ResourceLabel[];
};
