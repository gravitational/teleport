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
  github?: GithubConfig;
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

type GithubConfig = {
  /** enterprise_server_host allows joining from GitHub Actions workflows in a GitHub Enterprise Server instance. For normal situations, where you are using github.com, this option should be omitted. If you are using GHES, this value should be configured to the hostname of your GHES instance. */
  enterprise_server_host: string | null;
  /** static_jwks allows the JSON Web Key Set (JWKS) used to verify the token issued by GitHub Actions to be overridden. This can be used in scenarios where the Teleport Auth Service is unable to reach a GHES server. */
  static_jwks: string | null;
  /** enterprise_slug allows the slug of a GitHub Enterprise organisation to be included in the expected issuer of the OIDC tokens. This is for compatibility with the include_enterprise_slug option in GHE. */
  enterprise_slug: string | null;
  /** allow is an array of rule configurations for which GitHub Actions workflows should be allowed to join */
  allow: GithubRules[];
};

export type GithubRules = {
  /** repository is a fully qualified (e.g. including the owner) name of a GitHub repository. */
  repository: string | null;
  /** repository_owner is the name of an organization or user that a repository belongs to. */
  repository_owner: string | null;
  /** workflow is the exact name of a workflow as configured in the GitHub Action workflow YAML file. */
  workflow: string | null;
  /** environment is the environment associated with the GitHub Actions run. If no environment is configured for the GitHub Actions run, this will be empty. */
  environment: string | null;
  /** actor is the GitHub username that caused the GitHub Actions run, whether by committing or by directly despatching the workflow. */
  actor: string | null;
  /** ref is the git ref that triggered the action run. */
  ref: string | null;
  /** ref_type is the type of the git ref that triggered the action run. */
  ref_type: string | null;
  /** sub is a concatenated string of various attributes of the workflow run. GitHub explains the format of this string at: https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect#example-subject-claims */
  sub: string | null;
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
  github?: GithubConfig;
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
