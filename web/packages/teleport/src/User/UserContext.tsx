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

import React, {
  createContext,
  useCallback,
  PropsWithChildren,
  useContext,
  useRef,
  useEffect,
  useState,
} from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { Indicator } from 'design';

import { StyledIndicator } from 'teleport/Main';

import * as service from 'teleport/services/userPreferences';
import cfg from 'teleport/config';

import storage, { KeysEnum } from 'teleport/services/localStorage';

import {
  deprecatedThemeToThemePreference,
  ThemePreference,
} from 'teleport/services/userPreferences/types';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import type {
  UserClusterPreferences,
  UserPreferences,
} from 'teleport/services/userPreferences/types';

export interface UserContextValue {
  preferences: UserPreferences;
  updatePreferences: (preferences: Partial<UserPreferences>) => Promise<void>;
  updateClusterPinnedResources: (
    clusterId: string,
    pinnedResources: string[]
  ) => Promise<void>;
  getClusterPinnedResources: (clusterId: string) => Promise<string[]>;
}

export const UserContext = createContext<UserContextValue>(null);

export function useUser(): UserContextValue {
  return useContext(UserContext);
}

export function UserContextProvider(props: PropsWithChildren<unknown>) {
  const { attempt, run } = useAttempt('processing');
  // because we have to update cluster preferences with itself during the update
  // we useRef here to prevent infinite rerenders
  const clusterPreferences = useRef<Record<string, UserClusterPreferences>>({});

  const [preferences, setPreferences] = useState<UserPreferences>(
    makeDefaultUserPreferences()
  );

  const getClusterPinnedResources = useCallback(async (clusterId: string) => {
    if (clusterPreferences.current[clusterId]) {
      // we know that pinned resources is supported because we've already successfully
      // fetched their pinned resources once before
      localStorage.removeItem(KeysEnum.PINNED_RESOURCES_NOT_SUPPORTED);
      return clusterPreferences.current[clusterId].pinnedResources;
    }
    const prefs = await service.getUserClusterPreferences(clusterId);
    clusterPreferences.current[clusterId] = prefs;
    return prefs.pinnedResources;
  }, []);

  const updateClusterPinnedResources = async (
    clusterId: string,
    pinnedResources: string[]
  ) => {
    if (!clusterPreferences.current[clusterId]) {
      clusterPreferences.current[clusterId] = { pinnedResources: [] };
    }
    clusterPreferences.current[clusterId].pinnedResources = pinnedResources;

    return service.updateUserClusterPreferences(clusterId, {
      ...preferences,
      clusterPreferences: clusterPreferences.current[clusterId],
    });
  };

  async function loadUserPreferences() {
    const storedPreferences = storage.getUserPreferences();
    const theme = storage.getDeprecatedThemePreference();

    try {
      const preferences = await service.getUserPreferences();
      clusterPreferences.current[cfg.proxyCluster] =
        preferences.clusterPreferences;
      if (!storedPreferences) {
        // there are no mirrored user preferences in local storage so this is the first time
        // the user has requested their preferences in this browser session

        // if there is a legacy theme preference, update the preferences with it and remove it
        if (theme) {
          preferences.theme = deprecatedThemeToThemePreference(theme);

          if (preferences.theme !== ThemePreference.Light) {
            // the light theme is the default, so only update the backend if it is not light
            updatePreferences(preferences);
          }

          storage.clearDeprecatedThemePreference();
        }
      }

      setPreferences(preferences);
      storage.setUserPreferences(preferences);
    } catch (err) {
      if (storedPreferences) {
        setPreferences(storedPreferences);

        return;
      }

      if (theme) {
        setPreferences({
          ...preferences,
          theme: deprecatedThemeToThemePreference(theme),
        });
      }
    }
  }

  function updatePreferences(newPreferences: Partial<UserPreferences>) {
    const nextPreferences = {
      ...preferences,
      ...newPreferences,
      assist: {
        ...preferences.assist,
        ...newPreferences.assist,
      },
      onboard: {
        ...preferences.onboard,
        ...newPreferences.onboard,
      },
      unifiedResourcePreferences: {
        ...preferences.unifiedResourcePreferences,
        ...newPreferences.unifiedResourcePreferences,
      },
      // updatePreferences only update the root cluster so we can only pass cluster
      // preferences from the root cluster
      clusterPreferences: clusterPreferences.current[cfg.proxyCluster],
    } as UserPreferences;
    setPreferences(nextPreferences);
    storage.setUserPreferences(nextPreferences);

    return service.updateUserPreferences(nextPreferences);
  }

  useEffect(() => {
    function receiveMessage(event: StorageEvent) {
      if (!event.newValue || event.key !== KeysEnum.USER_PREFERENCES) {
        return;
      }

      setPreferences(JSON.parse(event.newValue));
    }

    storage.subscribe(receiveMessage);

    return () => storage.unsubscribe(receiveMessage);
  }, []);

  useEffect(() => {
    run(loadUserPreferences);
  }, []);

  if (attempt.status === 'processing') {
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    );
  }

  return (
    <UserContext.Provider
      value={{
        preferences,
        updatePreferences,
        getClusterPinnedResources,
        updateClusterPinnedResources,
      }}
    >
      {props.children}
    </UserContext.Provider>
  );
}
