/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  UnifiedResourcesTab,
  UnifiedResourcesViewMode,
} from 'shared/components/UnifiedResources';

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { ViewMode } from 'teleport/Assist/types';

import { ThemePreference } from 'teleport/services/userPreferences/types';

import { KeysEnum } from '../localStorage';

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
      // TODO (avatus) DELETE IN 15
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
      defaultTab: UnifiedResourcesTab.All,
      viewMode: UnifiedResourcesViewMode.Card,
    },
    clusterPreferences: makeDefaultUserClusterPreferences(),
  };
}

export function makeDefaultUserClusterPreferences(): UserClusterPreferences {
  return {
    pinnedResources: [],
  };
}
