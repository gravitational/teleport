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

import { GitServer } from 'web/packages/teleport/src/services/gitServers';

import type { Platform } from 'design/platform';
import {
  IncludedResourceMode,
  ResourceHealthStatus,
} from 'shared/components/UnifiedResources';

import { App } from 'teleport/services/apps';
import { Database } from 'teleport/services/databases';
import { Desktop } from 'teleport/services/desktops';
import { Kube } from 'teleport/services/kube';
import { Node } from 'teleport/services/nodes';
import { UserGroup } from 'teleport/services/userGroups';

import type { MfaChallengeResponse } from '../mfa';

export type UnifiedResource =
  | App
  | Database
  | Node
  | Kube
  | Desktop
  | UserGroup
  | GitServer;

export type UnifiedResourceKind = UnifiedResource['kind'];

export type ResourcesResponse<T> = {
  //TODO(gzdunek): Rename to items.
  agents: T[];
  startKey?: string;
  totalCount?: number;
};

export type ResourceLabel = {
  name: string;
  value: string;
};

export type ResourceFilter = {
  /** query is query expression using the predicate language. */
  query?: string;
  /** search contains search words/phrases separated by space. */
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
  pinnedOnly?: boolean;
  searchAsRoles?: '' | 'yes';
  includedResourceMode?: IncludedResourceMode;
  statuses?: ResourceHealthStatus[];
  // TODO(bl-nero): Remove this once filters are expressed as advanced search.
  kinds?: string[];
};

export type SortType = {
  fieldName: string;
  dir: SortDir;
};

export type SortDir = 'ASC' | 'DESC';

/**
 * ResourceIdKind are the same id constants used to mark the type of
 * resource in the backend.
 *
 * These consts are expected for various resource requests:
 *   - search based access requests
 *   - diagnose connection requests
 */
export type ResourceIdKind =
  | 'node'
  | 'app'
  | 'db'
  | 'kube_cluster'
  | 'user_group'
  | 'windows_desktop'
  | 'saml_idp_service_provider'
  | 'aws_ic_account_assignment'
  | 'git_server';

export type AccessRequestScope =
  | 'my_requests'
  | 'needs_review'
  | 'reviewed'
  | '';

export type ConnectionDiagnostic = {
  /** id is the identifier of the connection diagnostic. */
  id: string;
  /** success is whether the connection was successful */
  success: boolean;
  /** message is the diagnostic summary */
  message: string;
  /** traces contains multiple checkpoints results */
  traces: ConnectionDiagnosticTrace[];
};

/** ConnectionDiagnosticTrace describes a trace of a connection diagnostic */
export type ConnectionDiagnosticTrace = {
  traceType: string;
  status: 'success' | 'failed';
  details: string;
  error?: string;
};

/**
 * ConnectionDiagnosticRequest contains
 * - the identification of the resource kind and resource name to test
 * - additional paramenters which depend on the actual kind of resource to test
 * As an example, for SSH Node it also includes the User/Principal that will be used to login
 */
export type ConnectionDiagnosticRequest = {
  resourceKind: ResourceIdKind; //`json:"resource_kind"`
  resourceName: string; //`json:"resource_name"`
  sshPrincipal?: string; //`json:"ssh_principal"`
  /**
   * An optional field which describes whether the SSH principal was chosen manually by the user or
   * automatically. Used in Connect My Computer which automatically picks the principal if there's
   * only a single login available in the Connect My Computer role.
   */
  sshPrincipalSelectionMode?: 'manual' | 'auto'; //`json:"ssh_principal_selection_mode"`
  /**
   * An optional field which describes the platform the SSH agent runs on.
   */
  sshNodeOS?: Platform; // `json:"ssh_node_os"`
  /**
   * An optional field which which describes how an SSH agent was installed.
   * The value must match one of the consts defined in lib/client/conntest/connection_tester.go.
   */
  sshNodeSetupMethod?: 'script' | 'connect_my_computer'; // `json:"ssh_node_setup_method"`
  kubeImpersonation?: KubeImpersonation; // `json:"kubernetes_impersonation"`
  dbTester?: DatabaseTester;
  mfaAuthnResponse?: MfaChallengeResponse;
};

export type KubeImpersonation = {
  namespace: string; // `json:"kubernetes_namespace"`
  /**
   * The Kubernetes user to impersonate for this request.
   * Optional - If multiple values are configured the user must select one
   * otherwise the request will return an error.
   */
  user?: string; // `json:"kubernetes_impersonation.kubernetes_user"`
  /**
   * The Kubernetes groups to impersonate for this request.
   * Optional - If not specified it use all configured groups.
   * When KubernetesGroups is specified, KubernetesUser must be provided
   * as well.
   */
  groups?: string[]; // `json:"kubernetes_impersonation.kubernetes_groups"
};

export type DatabaseTester = {
  user?: string; // `json:"database_user"`
  name?: string; // `json:"database_name"`
};
