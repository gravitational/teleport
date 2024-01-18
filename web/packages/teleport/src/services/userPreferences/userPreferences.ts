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

import * as unifiedResourcePreferences from 'shared/services/unifiedResourcePreferences';

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import { ViewMode } from 'teleport/Assist/types';

import { KeysEnum } from '../storageService';

import { ThemePreference } from './types';

import type {
  GetUserPreferencesResponse,
  UserClusterPreferences,
  UserPreferences,
} from 'teleport/services/userPreferences/types';

export async function getUserPreferences() {
  const res: GetUserPreferencesResponse = await api.get(
    cfg.api.userPreferencesPath
  );

  return res;
}

export async function getUserClusterPreferences(clusterId: string) {
  return await api
    .get(cfg.getUserClusterPreferencesUrl(clusterId))
    .then(res => {
      // TODO (avatus) DELETE IN 16
      // this item is used to disabled the pinned resources button if they
      // haven't upgraded to 14.1.0 yet. Anything lower than 14 doesn't matter
      // because the unified resource view isn't enabled so pinning isn't there either
      localStorage.removeItem(KeysEnum.PINNED_RESOURCES_NOT_SUPPORTED);
      return res;
    })
    .catch(res => {
      if (res.response?.status === 403 || res.response?.status === 404) {
        localStorage.setItem(KeysEnum.PINNED_RESOURCES_NOT_SUPPORTED, 'true');
        // we handle this null error in the user context where we cache cluster
        // preferences. We want to fail gracefully here and use our "not supported"
        // message instead.
        return null;
      }
      // return all other errors here
      return res;
    });
}

export function updateUserClusterPreferences(
  clusterId: string,
  preferences: UserPreferences
) {
  return api.put(cfg.getUserClusterPreferencesUrl(clusterId), preferences);
}

export function updateUserPreferences(preferences: Partial<UserPreferences>) {
  return api.put(cfg.api.userPreferencesPath, preferences);
}

export function makeDefaultUserPreferences(): UserPreferences {
  return {
    theme: ThemePreference.Light,
    assist: {
      viewMode: ViewMode.Docked,
      preferredLogins: [],
    },
    onboard: {
      preferredResources: [],
      marketingParams: {
        campaign: '',
        source: '',
        medium: '',
        intent: '',
      },
    },
    unifiedResourcePreferences: {
      defaultTab: unifiedResourcePreferences.DefaultTab.DEFAULT_TAB_ALL,
      viewMode: unifiedResourcePreferences.ViewMode.VIEW_MODE_CARD,
      labelsViewMode:
        unifiedResourcePreferences.LabelsViewMode.LABELS_VIEW_MODE_COLLAPSED,
    },
    clusterPreferences: makeDefaultUserClusterPreferences(),
  };
}

export function makeDefaultUserClusterPreferences(): UserClusterPreferences {
  return {
    pinnedResources: [],
  };
}
