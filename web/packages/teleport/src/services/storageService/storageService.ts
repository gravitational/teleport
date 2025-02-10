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

import { OnboardUserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { RecentHistoryItem } from 'teleport/Navigation/RecentHistory';
import { OnboardDiscover } from 'teleport/services/user';
import {
  BackendUserPreferences,
  convertBackendUserPreferences,
  isBackendUserPreferences,
} from 'teleport/services/userPreferences/userPreferences';
import { BearerToken } from 'teleport/services/websession';
import { getPrefersDark } from 'teleport/ThemeProvider';
import type { RecommendFeature } from 'teleport/types';

import { CloudUserInvites, KeysEnum, LocalStorageSurvey } from './types';

// This is an array of local storage `KeysEnum` that are kept when a user logs out
const KEEP_LOCALSTORAGE_KEYS_ON_LOGOUT = [
  KeysEnum.THEME,
  KeysEnum.USER_PREFERENCES,
  KeysEnum.ACCESS_LIST_PREFERENCES,
  KeysEnum.RECOMMEND_FEATURE,
  KeysEnum.LICENSE_ACKNOWLEDGED,
  KeysEnum.USERS_NOT_EQUAL_TO_MAU_ACKNOWLEDGED,
  KeysEnum.USE_NEW_ROLE_EDITOR,
  KeysEnum.RECENT_HISTORY,
];

const RECENT_HISTORY_MAX_LENGTH = 10;

export const storageService = {
  clear() {
    Object.keys(window.localStorage).forEach(key => {
      const isAccessGraph = key.startsWith('tag_');

      if (!isAccessGraph && !KEEP_LOCALSTORAGE_KEYS_ON_LOGOUT.includes(key)) {
        window.localStorage.removeItem(key);
      }
    });
  },

  subscribe(fn: (e: StorageEvent) => void) {
    window.addEventListener('storage', fn);
  },

  unsubscribe(fn: (e: StorageEvent) => void) {
    window.removeEventListener('storage', fn);
  },

  getParsedJSONValue<T>(key: string, defaultValue: T): T {
    const item = window.localStorage.getItem(key);
    if (item) {
      return JSON.parse(item);
    }
    return defaultValue;
  },

  setBearerToken(token: BearerToken) {
    window.localStorage.setItem(KeysEnum.TOKEN, JSON.stringify(token));
  },

  getBearerToken(): BearerToken {
    return this.getParsedJSONValue(KeysEnum.TOKEN, null);
  },

  getAccessToken() {
    const bearerToken = this.getBearerToken();
    return bearerToken ? bearerToken.accessToken : null;
  },

  getSessionInactivityTimeout() {
    const bearerToken = this.getBearerToken();
    const time = Number(bearerToken.sessionInactiveTimeout);
    return time ? time : 0;
  },

  setLastActive(expiry: number) {
    window.localStorage.setItem(KeysEnum.LAST_ACTIVE, `${expiry}`);
  },

  getLastActive() {
    const time = Number(window.localStorage.getItem(KeysEnum.LAST_ACTIVE));
    return time ? time : 0;
  },

  setLoginTimeOnce() {
    const existingTime = window.localStorage.getItem(KeysEnum.LOGIN_TIME);
    // Only set the login time if it doesn't already exist.
    if (!existingTime) {
      window.localStorage.setItem(KeysEnum.LOGIN_TIME, `${Date.now()}`);
    }
  },

  getLoginTime(): Date {
    const time = Number(window.localStorage.getItem(KeysEnum.LOGIN_TIME));
    return time && !Number.isNaN(time) ? new Date(time) : new Date(0);
  },

  clearLoginTime() {
    window.localStorage.removeItem(KeysEnum.LOGIN_TIME);
  },

  // setOnboardDiscover persists states used to determine if a user should
  // be onboarded to use the discovery wizard or not. User should only
  // be onboarded once upon login.
  setOnboardDiscover(d: OnboardDiscover) {
    window.localStorage.setItem(KeysEnum.DISCOVER, JSON.stringify(d));
  },

  getOnboardDiscover(): OnboardDiscover {
    return this.getParsedJSONValue(KeysEnum.DISCOVER, null);
  },

  getUserPreferences(): UserPreferences {
    const preferences = window.localStorage.getItem(KeysEnum.USER_PREFERENCES);
    if (preferences) {
      const parsed: UserPreferences | BackendUserPreferences =
        JSON.parse(preferences);

      // TODO(ryan): DELETE in v17: remove reference to `BackendUserPreferences` - all
      //             locally stored copies of user preferences should be `UserPreferences` by then
      //             (they are updated on every login)
      if (isBackendUserPreferences(parsed)) {
        return convertBackendUserPreferences(parsed);
      }

      return parsed;
    }
    return null;
  },

  setUserPreferences(preferences: UserPreferences) {
    const json = JSON.stringify(preferences);

    window.localStorage.setItem(KeysEnum.USER_PREFERENCES, json);

    window.dispatchEvent(
      new StorageEvent('storage', {
        key: KeysEnum.USER_PREFERENCES,
        newValue: json,
      })
    );
  },

  getOnboardSurvey(): LocalStorageSurvey {
    return this.getParsedJSONValue(KeysEnum.ONBOARD_SURVEY, null);
  },

  setOnboardSurvey(survey: LocalStorageSurvey) {
    const json = JSON.stringify(survey);

    window.localStorage.setItem(KeysEnum.ONBOARD_SURVEY, json);
  },

  clearOnboardSurvey() {
    window.localStorage.removeItem(KeysEnum.ONBOARD_SURVEY);
  },

  getCloudUserInvites(): CloudUserInvites {
    return this.getParsedJSONValue(KeysEnum.CLOUD_USER_INVITES, null);
  },

  setCloudUserInvites(invites: CloudUserInvites) {
    const json = JSON.stringify(invites);

    window.localStorage.setItem(KeysEnum.CLOUD_USER_INVITES, json);
  },

  clearCloudUserInvites() {
    window.localStorage.removeItem(KeysEnum.CLOUD_USER_INVITES);
  },

  getThemePreference(): Theme {
    const userPreferences = storageService.getUserPreferences();
    if (userPreferences && userPreferences.theme !== Theme.UNSPECIFIED) {
      return userPreferences.theme;
    }

    const prefersDark = getPrefersDark();
    return prefersDark ? Theme.DARK : Theme.LIGHT;
  },

  getOnboardUserPreference(): OnboardUserPreferences {
    const userPreferences = storageService.getUserPreferences();
    if (userPreferences) {
      return userPreferences.onboard;
    }

    return {
      preferredResources: [],
      marketingParams: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
    };
  },

  getLicenseAcknowledged(): boolean {
    return (
      window.localStorage.getItem(KeysEnum.LICENSE_ACKNOWLEDGED) === 'true'
    );
  },

  setLicenseAcknowledged() {
    window.localStorage.setItem(KeysEnum.LICENSE_ACKNOWLEDGED, 'true');
  },

  getUsersMauAcknowledged(): boolean {
    return (
      window.localStorage.getItem(
        KeysEnum.USERS_NOT_EQUAL_TO_MAU_ACKNOWLEDGED
      ) === 'true'
    );
  },

  setUsersMAUAcknowledged() {
    window.localStorage.setItem(
      KeysEnum.USERS_NOT_EQUAL_TO_MAU_ACKNOWLEDGED,
      'true'
    );
  },

  broadcast(messageType, messageBody) {
    window.localStorage.setItem(messageType, messageBody);
    window.localStorage.removeItem(messageType);
  },

  // setRecommendFeature persists states used to determine if
  // given feature needs to be recommended to the user.
  // Currently, it only shows a red dot in the side navigation menu.
  setRecommendFeature(d: RecommendFeature) {
    window.localStorage.setItem(KeysEnum.RECOMMEND_FEATURE, JSON.stringify(d));
  },

  getFeatureRecommendationStatus(): RecommendFeature {
    return this.getParsedJSONValue(KeysEnum.RECOMMEND_FEATURE, null);
  },

  getAccessGraphEnabled(): boolean {
    return this.getParsedJSONValue(KeysEnum.ACCESS_GRAPH_ENABLED, false);
  },

  getAccessGraphSQLEnabled(): boolean {
    return this.getParsedJSONValue(KeysEnum.ACCESS_GRAPH_SQL_ENABLED, false);
  },

  getAccessGraphRoleTesterEnabled(): boolean {
    return this.getParsedJSONValue(
      KeysEnum.ACCESS_GRAPH_ROLE_TESTER_ENABLED,
      false
    );
  },

  getExternalAuditStorageCtaDisabled(): boolean {
    return this.getParsedJSONValue(
      KeysEnum.EXTERNAL_AUDIT_STORAGE_CTA_DISABLED,
      false
    );
  },

  disableExternalAuditStorageCta(): void {
    window.localStorage.setItem(
      KeysEnum.EXTERNAL_AUDIT_STORAGE_CTA_DISABLED,
      JSON.stringify(true)
    );
  },

  getUseNewRoleEditor(): boolean {
    return this.getParsedJSONValue(KeysEnum.USE_NEW_ROLE_EDITOR, true);
  },

  getIsTopBarView(): boolean {
    return this.getParsedJSONValue(KeysEnum.USE_TOP_BAR, false);
  },

  getRecentHistory(): RecentHistoryItem[] {
    return this.getParsedJSONValue(KeysEnum.RECENT_HISTORY, []);
  },

  addRecentHistoryItem(item: RecentHistoryItem): RecentHistoryItem[] {
    const history = storageService.getRecentHistory();
    const deduplicatedHistory = [...history];

    // Remove a duplicate item if it exists.
    const existingDuplicateIndex = history.findIndex(
      historyItem => historyItem.route === item.route
    );
    if (existingDuplicateIndex !== -1) {
      deduplicatedHistory.splice(existingDuplicateIndex, 1);
    }

    const newHistory = [item, ...deduplicatedHistory].slice(
      0,
      RECENT_HISTORY_MAX_LENGTH
    );

    window.localStorage.setItem(
      KeysEnum.RECENT_HISTORY,
      JSON.stringify(newHistory)
    );

    return newHistory;
  },

  removeRecentHistoryItem(route: string): RecentHistoryItem[] {
    const history = storageService.getRecentHistory();
    const newHistory = history.filter(item => item.route !== route);

    window.localStorage.setItem(
      KeysEnum.RECENT_HISTORY,
      JSON.stringify(newHistory)
    );

    return newHistory;
  },
};
