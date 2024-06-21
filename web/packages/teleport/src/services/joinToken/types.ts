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
  safeName: string;
  isStatic: boolean;
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
  content: string;
};

// JoinRole defines built-in system roles and are roles associated with
// a join token and will be granted to the user of the join token.
// Same hard-coded value as the backend:
// - 'App' is a role for an app proxy in the cluster
// - 'Db' is a role for a database proxy in the cluster
// - 'Kube' is a role for a kube service
// - 'Node' is a role for a node in the cluster
// - 'WindowsDesktop' is a role for a windows desktop service.
// - 'Discovery' is a role for a discovery service.
export type JoinRole =
  | 'App'
  | 'Node'
  | 'Db'
  | 'Kube'
  | 'WindowsDesktop'
  | 'Discovery';

// JoinMethod is the method used for new nodes to join the cluster.
// Same hard-corded value as the backend.
// - 'token' is the default method, where nodes join the cluster by
//   presenting a secret token.
export type JoinMethod = 'token' | 'ec2' | 'iam' | 'github';

// JoinRule is a rule that a joining node must match in order to use the
// associated token.
export type JoinRule = {
  awsAccountId: string;
  // awsArn is used for the IAM join method.
  awsArn?: string;
};

export type JoinTokenRequest = {
  // roles is a list of join roles, since there can be more than
  // one role associated with a token.
  roles: JoinRole[];
  // rules is a list of allow rules associated with the join token
  // and the node using this token must match one of the rules.
  rules?: JoinRule[];
  // suggestedAgentMatcherLabels is a set of labels to be used by agents to match
  // on resources. When an agent uses this token, the agent should
  // monitor resources that match those labels. For databases, this
  // means adding the labels to `db_service.resources.labels`.
  suggestedAgentMatcherLabels?: ResourceLabel[];
  method?: JoinMethod;
};
