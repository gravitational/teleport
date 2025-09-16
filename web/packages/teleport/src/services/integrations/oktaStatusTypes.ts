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

/**
 * OktaPluginSyncStatusCode indicates the possible states of an Okta
 * synchronization service.
 */
export enum PluginOktaSyncStatusCode {
  /**
   * is the status code zero value,
   * indicating that the service has not yet reported a status code.
   */
  Unspecified = 0,
  /**
   * indicates that the service is running without error
   */
  Success = 1,
  /**
   * indicates that the service is currently in an error state.
   */
  Error = 2,
}

/**
 * Contains statistics about the various sub-services in the Okta
 * integration
 */
export type PluginStatusOkta = {
  ssoDetails?: OktaSsoDetails;
  appGroupSyncDetails?: OktaAppGroupSyncDetails;
  usersSyncDetails?: OktaUserSyncDetails;
  accessListsSyncDetails?: OktaAccessListSyncDetails;
  scimDetails?: OktaScimDetails;
};

export type OktaSsoDetails = {
  enabled: boolean;
  appId: string;
  appName: string;
};

export type OktaAppGroupSyncDetails = {
  enabled: boolean;
  statusCode: PluginOktaSyncStatusCode;
  lastSuccess: Date;
  lastFailed: Date;
  numApps: number;
  numGroups: number;
  /**
   * Error contains a textual description of the reason the last synchronization
   * failed. Only valid when StatusCode is OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR.
   */
  error: string;
};

export type OktaUserSyncDetails = {
  enabled: boolean;
  statusCode: PluginOktaSyncStatusCode;
  lastSuccess: Date;
  lastFailed: Date;
  numUsers: number;
  /**
   * Error contains a textual description of the reason the last synchronization
   * failed. Only valid when StatusCode is OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR.
   */
  error: string;
};

export type OktaAccessListSyncDetails = {
  enabled: boolean;
  statusCode: PluginOktaSyncStatusCode;
  lastSuccess: Date;
  lastFailed: Date;
  numApps: number;
  numGroups: number;
  appFilters: string[];
  groupFilters: string[];
  /**
   * Error contains a textual description of the reason the last synchronization
   * failed. Only valid when StatusCode is OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR.
   */
  error: string;
};

export type OktaScimDetails = {
  enabled: boolean;
};
