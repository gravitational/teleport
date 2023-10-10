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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { ViewMode } from 'teleport/Assist/types';
import { ThemePreference } from 'teleport/services/userPreferences/types';

import type {
  GetUserClusterPreferencesResponse,
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
  const res: GetUserClusterPreferencesResponse = await api.get(
    cfg.getUserClusterPreferencesUrl(clusterId)
  );

  return res;
}

export function updateUserClusterPreferences(
  clusterId: string,
  preferences: Partial<UserPreferences>
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
    clusterPreferences: makeDefaultUserClusterPreferences(),
  };
}

export function makeDefaultUserClusterPreferences(): UserClusterPreferences {
  return {
    pinnedResources: [],
  };
}
