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

import { ResourceLabel } from '../agents';

export type JoinToken = {
  id: string;
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
// - 'ec2' is a method where node will join with the EC2 join method
// - 'iam' is a method where node will join with the IAM join method
export type JoinMethod = 'token' | 'ec2' | 'iam';

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
