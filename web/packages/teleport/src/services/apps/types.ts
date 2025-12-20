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

import { AppSubKind } from 'shared/services';
import { AwsRole } from 'shared/services/apps';
import { ComponentFeatureID } from 'shared/utils/componentFeatures';

import { ResourceLabel } from 'teleport/services/agents';
import type { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

/**
 * Describes what cloud instance the app was discovered from.
 *
 * Values are same consts used in backend and letter casing matters:
 * https://github.com/gravitational/teleport/blob/095e8e7b12d7be546c34eb15e4e562693cc81338/api/types/constants.go#L947
 */
export enum CloudInstance {
  Azure = 'Azure',
  Gcp = 'GCP',
  Aws = 'AWS',
}

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
  useAnyProxyPublicAddr?: boolean;
  awsRoles: AwsRole[];
  awsConsole: boolean;
  requiresRequest?: boolean;
  isTcp?: boolean;
  /**
   * This field is equivalent to `isCloud` field but this field
   * specifies what cloud instance is used.
   */
  cloudInstance?: CloudInstance;
  isCloud?: boolean;
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
   * samlAppLaunchUrl contains service provider specific authentication
   * endpoints where user should be launched to start SAML authentication.
   */
  samlAppLaunchUrls?: SamlAppLaunchUrl[];
  /**
   * mcp contains MCP server specific configurations.
   */
  mcp?: AppMCP;
  /**
   * supportedFeatureIds contains component feature IDs supported by
   * both the App and all required back-end components.
   */
  supportedFeatureIds?: ComponentFeatureID[];
}

export type UserGroupAndDescription = {
  name: string;
  description: string;
};

/** AppSubKind defines names of SubKind for App resource. */
export {
  /*
   * @deprecated Import AppSubKind from 'shared/services' instead.
   */
  AppSubKind,
} from 'shared/services';

/**
 * PermissionSet defines an AWS IAM Identity Center permission set that
 * is available to an App.
 */
export type PermissionSet = {
  /*
   * name is a friendly permission set name
   * eg: AdministratorAccess
   */
  name: string;
  /*
   * arn is a permission set ARN
   * starts with "arn:aws:sso:::"
   */
  arn: string;
  /*
   * assignmentId is an account assignment ID.
   * It is found in the format <awsAccountID>--<friendly-name>
   * eg: 1234--AdministratorAccess
   */
  assignmentId: string;
  requiresRequest?: boolean;
};

/**
 * SamlAppLaunchUrl contains service provider specific authentication
 * endpoint where user should be launched to start SAML authentication.
 */
export type SamlAppLaunchUrl = {
  /* launch URL. */
  url: string;
  /* friendly name of the URL. */
  friendlyName?: string;
};

/**
 * AppMCP contains MCP server specific configurations.
 */
export type AppMCP = {
  /** Command to launch stdio-based MCP servers. */
  command: string;
  /** Args to execute with the command. */
  args?: string[];
  /**
   * The host user account under which the command will be
   * executed. Required for stdio-based MCP servers.
   */
  runAsHostUser: string;
};
