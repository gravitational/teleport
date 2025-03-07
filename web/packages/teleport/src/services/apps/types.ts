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

import { AwsRole } from 'shared/services/apps';

import { ResourceLabel } from 'teleport/services/agents';
import type { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

export interface App {
  kind: 'app';
  id: string;
  name: string;
  description: string;
  uri: string;
  publicAddr: string;
  labels: ResourceLabel[];
  clusterId: string;
  launchUrl: string;
  fqdn: string;
  awsRoles: AwsRole[];
  awsConsole: boolean;
  requiresRequest?: boolean;
  isCloudOrTcpEndpoint?: boolean;
  // addrWithProtocol can either be a public address or
  // if public address wasn't defined, fallback to uri
  addrWithProtocol?: string;
  friendlyName?: string;
  userGroups: UserGroupAndDescription[];
  // samlApp is whether the application is a SAML Application (Service Provider).
  samlApp: boolean;
  // samlAppSsoUrl is the URL that triggers IdP-initiated SSO for SAML Application;
  samlAppSsoUrl?: string;
  // samlAppPreset is used to identify SAML service provider preset type.
  samlAppPreset?: SamlServiceProviderPreset;
  // Integration is the integration name that must be used to access this Application.
  // Only applicable to AWS App Access.
  integration?: string;
  /** subKind is subKind of an App. */
  subKind?: AppSubKind;
  /**
   * permissionSets is a list of AWS IAM Identity Center permission sets
   * available for this App. The value is only populated if the app SubKind is
   * aws_ic_account.
   */
  permissionSets?: PermissionSet[];
  /**
   * SamlAppLaunchUrl contains service provider specific authenticaiton
   * endpoints where user should be launched to start SAML authentication.
   */
  samlAppLaunchUrls?: SamlAppLaunchUrl[];
}

export type UserGroupAndDescription = {
  name: string;
  description: string;
};

/** AppSubKind defines names of SubKind for App resource. */
export enum AppSubKind {
  AwsIcAccount = 'aws_ic_account',
}

/**
 * PermissionSet defines an AWS IAM Identity Center permission set that
 * is available to an App.
 */
export type PermissionSet = {
  /** name is a permission set name */
  name: string;
  /** arn is a permission set ARN */
  arn: string;
  /** assignmentId is an account assignment ID. */
  assignmentId: string;
};

/**
 * SamlAppLaunchUrl contains service provider specific authenticaiton
 * endpoint where user should be launched to start SAML authentication.
 */
export type SamlAppLaunchUrl = {
  /* launch URL. */
  url: string;
  /* friendly name of the URL. */
  friendlyName?: string;
};
