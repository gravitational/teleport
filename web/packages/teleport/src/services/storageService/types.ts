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

export const KeysEnum = {
  TOKEN: 'grv_teleport_token',
  TOKEN_RENEW: 'grv_teleport_token_renew',
  LAST_ACTIVE: 'grv_teleport_last_active',
  DISCOVER: 'grv_teleport_discover',
  THEME: 'grv_teleport_ui_theme',
  USER_PREFERENCES: 'grv_teleport_user_preferences',
  ONBOARD_SURVEY: 'grv_teleport_onboard_survey',
  RECOMMEND_FEATURE: 'grv_recommend_feature',
  CLOUD_USER_INVITES: 'grv_teleport_cloud_user_invites',
  ACCESS_GRAPH_SEARCH_MODE: 'grv_teleport_access_graph_search_mode',
  ACCESS_GRAPH_QUERY: 'grv_teleport_access_graph_query',
  ACCESS_GRAPH_ENABLED: 'grv_teleport_access_graph_enabled',
  ACCESS_GRAPH_SQL_ENABLED: 'grv_teleport_access_graph_sql_enabled',
  EXTERNAL_AUDIT_STORAGE_CTA_DISABLED:
    'grv_teleport_external_audit_storage_disabled',
  LICENSE_ACKNOWLEDGED: 'grv_teleport_license_acknowledged',

  LOCAL_NOTIFICATION_STATES: 'grv_teleport_notification_states',
};

// SurveyRequest is the request for sending data to the back end
export type SurveyRequest = {
  companyName: string;
  employeeCount: string;
  resources: Array<string>;
  role: string;
  team: string;
};

// LocalStorageSurvey is the SurveyRequest type defined in Enterprise
export type LocalStorageSurvey = SurveyRequest & {
  clusterResources: Array<number>;
  marketingParams: LocalStorageMarketingParams;
};

// LocalStorageMarketingParams is the MarketingParams type defined in Enterprise
export type LocalStorageMarketingParams = {
  campaign: string;
  source: string;
  medium: string;
  intent: string;
};

// CloudUserInvites is a set of users and roles which should be submitted after
// initial login.
export type CloudUserInvites = {
  recipients: Array<string>;
  roles: Array<string>;
};
