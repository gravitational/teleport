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

import { DeprecatedThemeOption } from 'design/theme/types';

import { BearerToken } from 'teleport/services/websession';
import { OnboardDiscover } from 'teleport/services/user';

import {
  OnboardUserPreferences,
  ThemePreference,
  UserPreferences,
} from 'teleport/services/userPreferences/types';

import { CloudUserInvites, KeysEnum, LocalStorageSurvey } from './types';

import type { RecommendFeature } from 'teleport/types';

// This is an array of local storage `KeysEnum` that are kept when a user logs out
const KEEP_LOCALSTORAGE_KEYS_ON_LOGOUT = [
  KeysEnum.THEME,
  KeysEnum.SHOW_ASSIST_POPUP,
  KeysEnum.USER_PREFERENCES,
  KeysEnum.RECOMMEND_FEATURE,
];

export const storageService = {
  clear() {
    Object.keys(window.localStorage).forEach(key => {
      if (!KEEP_LOCALSTORAGE_KEYS_ON_LOGOUT.includes(key)) {
        window.localStorage.removeItem(key);
      }
    });
  },

  subscribe(fn) {
    window.addEventListener('storage', fn);
  },

  unsubscribe(fn) {
    window.removeEventListener('storage', fn);
  },

  setBearerToken(token: BearerToken) {
    window.localStorage.setItem(KeysEnum.TOKEN, JSON.stringify(token));
  },

  getBearerToken(): BearerToken {
    const item = window.localStorage.getItem(KeysEnum.TOKEN);
    if (item) {
      return JSON.parse(item);
    }

    return null;
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

  // setOnboardDiscover persists states used to determine if a user should
  // be onboarded to use the discovery wizard or not. User should only
  // be onboarded once upon login.
  setOnboardDiscover(d: OnboardDiscover) {
    window.localStorage.setItem(KeysEnum.DISCOVER, JSON.stringify(d));
  },

  getOnboardDiscover(): OnboardDiscover {
    const item = window.localStorage.getItem(KeysEnum.DISCOVER);
    if (item) {
      return JSON.parse(item);
    }
    return null;
  },

  getUserPreferences(): UserPreferences {
    const preferences = window.localStorage.getItem(KeysEnum.USER_PREFERENCES);
    if (preferences) {
      return JSON.parse(preferences);
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
    const survey = window.localStorage.getItem(KeysEnum.ONBOARD_SURVEY);
    if (survey) {
      return JSON.parse(survey);
    }
    return null;
  },

  setOnboardSurvey(survey: LocalStorageSurvey) {
    const json = JSON.stringify(survey);

    window.localStorage.setItem(KeysEnum.ONBOARD_SURVEY, json);
  },

  clearOnboardSurvey() {
    window.localStorage.removeItem(KeysEnum.ONBOARD_SURVEY);
  },

  getCloudUserInvites(): CloudUserInvites {
    const invites = window.localStorage.getItem(KeysEnum.CLOUD_USER_INVITES);
    if (invites) {
      return JSON.parse(invites);
    }
    return null;
  },

  setCloudUserInvites(invites: CloudUserInvites) {
    const json = JSON.stringify(invites);

    window.localStorage.setItem(KeysEnum.CLOUD_USER_INVITES, json);
  },

  clearCloudUserInvites() {
    window.localStorage.removeItem(KeysEnum.CLOUD_USER_INVITES);
  },

  getThemePreference(): ThemePreference {
    const userPreferences = storageService.getUserPreferences();
    if (userPreferences) {
      return userPreferences.theme;
    }

    const theme = this.getDeprecatedThemePreference();
    if (theme) {
      return theme === 'light' ? ThemePreference.Light : ThemePreference.Dark;
    }

    return ThemePreference.Light;
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

  // DELETE IN 15 (ryan)
  getDeprecatedThemePreference(): DeprecatedThemeOption {
    return window.localStorage.getItem(KeysEnum.THEME) as DeprecatedThemeOption;
  },

  // TODO(ryan): remove in v15
  clearDeprecatedThemePreference() {
    window.localStorage.removeItem(KeysEnum.THEME);
  },

  arePinnedResourcesDisabled(): boolean {
    return (
      window.localStorage.getItem(KeysEnum.PINNED_RESOURCES_NOT_SUPPORTED) ===
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
    const item = window.localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    if (item) {
      return JSON.parse(item);
    }
    return null;
  },

  getAccessGraphEnabled(): boolean {
    const item = window.localStorage.getItem(KeysEnum.ACCESS_GRAPH_ENABLED);
    if (item) {
      return JSON.parse(item);
    }
    return false;
  },

  getAccessGraphSQLEnabled(): boolean {
    const item = window.localStorage.getItem(KeysEnum.ACCESS_GRAPH_SQL_ENABLED);
    if (item) {
      return JSON.parse(item);
    }
    return false;
  },

  getExternalAuditStorageCtaDisabled(): boolean {
    const item = window.localStorage.getItem(
      KeysEnum.EXTERNAL_AUDIT_STORAGE_CTA_DISABLED
    );
    if (item) {
      return JSON.parse(item);
    }
    return false;
  },

  disableExternalAuditStorageCta(): void {
    window.localStorage.setItem(
      KeysEnum.EXTERNAL_AUDIT_STORAGE_CTA_DISABLED,
      JSON.stringify(true)
    );
  },
};
