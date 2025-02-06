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

import { ClusterUserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/cluster_preferences_pb';
import { DiscoverResourcePreferences } from 'gen-proto-ts/teleport/userpreferences/v1/discover_resource_preferences_pb';
import { OnboardUserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { SideNavDrawerMode } from 'gen-proto-ts/teleport/userpreferences/v1/sidenav_preferences_pb';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';
import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  UnifiedResourcePreferences,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import { getPrefersDark } from 'teleport/ThemeProvider';

interface BackendClusterUserPreferences {
  pinnedResources?: string[];
}

export interface BackendUserPreferences {
  theme: Theme;
  sideNavDrawerMode: SideNavDrawerMode;
  onboard?: OnboardUserPreferences;
  clusterPreferences?: BackendClusterUserPreferences;
  unifiedResourcePreferences?: UnifiedResourcePreferences;
  discoverResourcePreferences?: DiscoverResourcePreferences;
}

export async function getUserPreferences(): Promise<UserPreferences> {
  const res: BackendUserPreferences = await api.get(
    cfg.api.userPreferencesPath
  );

  return convertBackendUserPreferences(res);
}

export async function getUserClusterPreferences(
  clusterId: string
): Promise<ClusterUserPreferences> {
  return await api
    .get(cfg.getUserClusterPreferencesUrl(clusterId))
    .then(convertBackendClusterUserPreferences);
}

export function updateUserClusterPreferences(
  clusterId: string,
  preferences: UserPreferences
) {
  return api.put(
    cfg.getUserClusterPreferencesUrl(clusterId),
    convertUserPreferences(preferences)
  );
}

export function updateUserPreferences(preferences: Partial<UserPreferences>) {
  return api.put(
    cfg.api.userPreferencesPath,
    convertUserPreferences(preferences as UserPreferences)
  );
}

export function makeDefaultUserPreferences(): UserPreferences {
  const prefersDark = getPrefersDark();
  return {
    theme: prefersDark ? Theme.DARK : Theme.LIGHT,
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
      defaultTab: DefaultTab.ALL,
      viewMode: ViewMode.CARD,
      labelsViewMode: LabelsViewMode.COLLAPSED,
      availableResourceMode: AvailableResourceMode.ALL,
    },
    clusterPreferences: makeDefaultUserClusterPreferences(),
    sideNavDrawerMode: SideNavDrawerMode.COLLAPSED,
  };
}

export function makeDefaultUserClusterPreferences(): ClusterUserPreferences {
  return {
    pinnedResources: {
      resourceIds: [],
    },
  };
}

export function convertUserPreferences(
  preferences: UserPreferences
): BackendUserPreferences {
  return {
    ...preferences,
    clusterPreferences: {
      ...preferences.clusterPreferences,
      pinnedResources:
        preferences.clusterPreferences?.pinnedResources?.resourceIds ?? [],
    },
  };
}

export function convertBackendUserPreferences(
  preferences: BackendUserPreferences
): UserPreferences {
  return {
    ...preferences,
    clusterPreferences: convertBackendClusterUserPreferences(
      preferences.clusterPreferences
    ),
    unifiedResourcePreferences: {
      availableResourceMode: AvailableResourceMode.NONE,
      ...preferences.unifiedResourcePreferences,
    },
  };
}

export function convertBackendClusterUserPreferences(
  clusterPreferences: BackendClusterUserPreferences
): ClusterUserPreferences {
  return {
    ...clusterPreferences,
    pinnedResources: {
      resourceIds: clusterPreferences?.pinnedResources ?? [],
    },
  };
}

export function isBackendUserPreferences(
  preferences: UserPreferences | BackendUserPreferences
): preferences is BackendUserPreferences {
  return Array.isArray(
    (preferences as BackendUserPreferences).clusterPreferences?.pinnedResources
  );
}
