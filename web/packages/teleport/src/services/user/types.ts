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

import { Cluster } from 'teleport/services/clusters';
import { MfaChallengeResponse } from 'teleport/services/mfa';

export type AuthType = 'local' | 'sso' | 'passwordless';

export interface AccessStrategy {
  type: 'optional' | 'always' | 'reason';
  prompt: string;
}

export interface AccessCapabilities {
  requestableRoles: string[];
  suggestedReviewers: string[];
  requireReason: boolean;
}

export interface UserContext {
  authType: AuthType;
  acl: Acl;
  username: string;
  cluster: Cluster;
  accessStrategy: AccessStrategy;
  accessCapabilities: AccessCapabilities;
  /**
   * ID of the access request from which additional roles to assume were
   * obtained for the current session.
   */
  accessRequestId?: string;
  allowedSearchAsRoles: string[];
  /** Indicates whether the user has a password set. */
  passwordState: PasswordState;
}

/**
 * Indicates whether a user has a password set. Corresponds to the PasswordState
 * protocol buffers enum.
 */
export enum PasswordState {
  PASSWORD_STATE_UNSPECIFIED = 0,
  PASSWORD_STATE_UNSET = 1,
  PASSWORD_STATE_SET = 2,
}

export interface Access {
  list: boolean;
  read: boolean;
  edit: boolean;
  create: boolean;
  remove: boolean;
}

export interface AccessWithUse extends Access {
  use: boolean;
}

export interface Acl {
  directorySharingEnabled: boolean;
  reviewRequests: boolean;
  desktopSessionRecordingEnabled: boolean;
  clipboardSharingEnabled: boolean;
  authConnectors: Access;
  trustedClusters: Access;
  roles: Access;
  recordedSessions: Access;
  activeSessions: Access;
  events: Access;
  users: Access;
  tokens: Access;
  appServers: Access;
  kubeServers: Access;
  accessRequests: Access;
  billing: Access;
  dbServers: Access;
  db: Access;
  desktops: Access;
  nodes: Access;
  connectionDiagnostic: Access;
  license: Access;
  download: Access;
  discoverConfigs: Access;
  plugins: Access;
  integrations: AccessWithUse;
  deviceTrust: Access;
  lock: Access;
  samlIdpServiceProvider: Access;
  accessList: Access;
  auditQuery: Access;
  securityReport: Access;
  externalAuditStorage: Access;
  accessGraph: Access;
  bots: Access;
  accessMonitoringRule: Access;
  contacts: Access;
  fileTransferAccess: boolean;
  gitServers: Access;
  accessGraphSettings: Access;
  botInstances: Access;
}

// AllTraits represent all the traits defined for a user.
export type AllUserTraits = Record<string, string[]>;

export type UserOrigin = 'okta' | 'saml' | 'scim';

export interface User {
  // name is the teleport username.
  name: string;
  // roles is the list of roles user is assigned to.
  roles: string[];
  // authType describes how the user authenticated
  // e.g. locally or with a SSO provider.
  authType?: string;
  // What kind of upstream IdP has the user come from?
  origin?: UserOrigin;
  // isLocal is true if json.authType was 'local'.
  isLocal?: boolean;
  // isBot is true if the user is a Bot User.
  isBot?: boolean;
  // traits are preset traits defined in Teleport, such as
  // logins, db_role etc. These traits are defiend in UserTraits interface.
  traits?: UserTraits;
  // allTraits contains both preset traits, as well as externalTraits
  // such as those created by external IdP attributes to roles mapping
  // or new values as set by Teleport admin.
  allTraits?: AllUserTraits;
}

// Backend does not allow User fields "traits" and "allTraits"
// both to be specified in the same request when creating or updating a user.
export enum ExcludeUserField {
  Traits = 'traits',
  AllTraits = 'allTraits',
}

// UserTraits contain fields that define traits for local accounts.
export interface UserTraits {
  // logins is the list of logins that this user is allowed to
  // start SSH sessions with.
  logins: string[];
  // databaseUsers is the list of db usernames that this user is
  // allowed to open db connections as.
  databaseUsers: string[];
  // databaseNames is the list of db names that this user can connect to.
  databaseNames: string[];
  // kubeUsers is the list of allowed kube logins.
  kubeUsers: string[];
  // kubeGroups is the list of allowed kube groups for a kube cluster.
  kubeGroups: string[];
  // windowsLogins is the list of logins that this user
  // is allowed to start desktop sessions.
  windowsLogins: string[];
  // awsRoleArns is a list of aws roles this user is allowed to assume.
  awsRoleArns: string[];
}

export interface ResetToken {
  value: string;
  username: string;
  expires: Date;
}

export type ResetPasswordType = 'invite' | 'password';

// OnboardDiscover describes states related to onboarding a
// user to using the discover wizard to add a resource.
export type OnboardDiscover = {
  // notified is a flag to indicate if user has been notified
  // that they can add a resource using the discover wizard.
  notified?: boolean;
  // hasResource is a flag to indicate if user has access to
  // any registered resource.
  hasResource: boolean;
  // hasVisited is a flag to indicate if user has visited the
  // discover page.
  hasVisited?: boolean;
};

export interface CreateUserVariables {
  user: User;
  excludeUserField: ExcludeUserField;
  mfaResponse?: MfaChallengeResponse;
}

export interface UpdateUserVariables {
  user: User;
  excludeUserField: ExcludeUserField;
}
