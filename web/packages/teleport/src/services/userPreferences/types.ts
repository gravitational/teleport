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

import { DeprecatedThemeOption } from 'design/theme';

import type { UnifiedResourcePreferences } from 'shared/services/unifiedResourcePreferences';

import type { AssistUserPreferences } from 'teleport/Assist/types';

export enum ThemePreference {
  Light = 1,
  Dark = 2,
}

export enum ClusterResource {
  RESOURCE_UNSPECIFIED = 0,
  RESOURCE_WINDOWS_DESKTOPS = 1,
  RESOURCE_SERVER_SSH = 2,
  RESOURCE_DATABASES = 3,
  RESOURCE_KUBERNETES = 4,
  RESOURCE_WEB_APPLICATIONS = 5,
}

export type MarketingParams = {
  campaign: string;
  source: string;
  medium: string;
  intent: string;
};

export type OnboardUserPreferences = {
  preferredResources: ClusterResource[];
  marketingParams: MarketingParams;
};

export interface UserPreferences {
  theme: ThemePreference;
  assist: AssistUserPreferences;
  onboard: OnboardUserPreferences;
  clusterPreferences: UserClusterPreferences;
  unifiedResourcePreferences: UnifiedResourcePreferences;
}

// UserClusterPreferences are user preferences that are
// different per cluster.
export interface UserClusterPreferences {
  // pinnedResources is an array of resource IDs.
  pinnedResources: string[];
}

export type GetUserClusterPreferencesResponse = UserClusterPreferences;
export type GetUserPreferencesResponse = UserPreferences;

export function deprecatedThemeToThemePreference(
  theme: DeprecatedThemeOption
): ThemePreference {
  switch (theme) {
    case 'light':
      return ThemePreference.Light;
    case 'dark':
      return ThemePreference.Dark;
  }
}
