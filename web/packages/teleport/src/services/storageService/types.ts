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
  ACCESS_GRAPH_IAC_ENABLED: 'grv_teleport_access_graph_iac_enabled',
  ACCESS_GRAPH_SQL_ENABLED: 'grv_teleport_access_graph_sql_enabled',
  ACCESS_GRAPH_ROLE_TESTER_ENABLED:
    'grv_teleport_access_graph_role_tester_enabled',
  ACCESS_LIST_PREFERENCES: 'grv_teleport_access_list_preferences',
  EXTERNAL_AUDIT_STORAGE_CTA_DISABLED:
    'grv_teleport_external_audit_storage_disabled',
  LICENSE_ACKNOWLEDGED: 'grv_teleport_license_acknowledged',
  USERS_NOT_EQUAL_TO_MAU_ACKNOWLEDGED:
    'grv_users_not_equal_to_mau_acknowledged',
  LOCAL_NOTIFICATION_STATES: 'grv_teleport_notification_states',
  RECENT_HISTORY: 'grv_teleport_sidenav_recent_history',
  LOGIN_TIME: 'grv_teleport_login_time',
  REMEMBERED_SSO_USERNAME: 'grv_teleport_remembered_sso_username',

  // TODO(bl-nero): Remove once the new role editor is in acceptable state.
  USE_NEW_ROLE_EDITOR: 'grv_teleport_use_new_role_editor',
  //TODO(rudream): Remove once sidenav implementation is complete.
  USE_TOP_BAR: 'grv_teleport_use_topbar',
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
