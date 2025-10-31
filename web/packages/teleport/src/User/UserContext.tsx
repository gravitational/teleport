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

import {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useLocation } from 'react-router-dom';

import { Indicator } from 'design';
import { ShieldCheck } from 'design/Icon';
import { ClusterUserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/cluster_preferences_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';
import { useToastNotifications } from 'shared/components/ToastNotification';
import useAttempt from 'shared/hooks/useAttemptNext';

import ecfg from 'e-teleport/config';
import {
  AccessListOrigin,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { useFetchPlugin } from 'e-teleport/services/plugins/hooks';
import cfg from 'teleport/config';
import { DiscoverResourcePreference } from 'teleport/Discover/SelectResource/utils/pins';
import { StyledIndicator } from 'teleport/Main';
import { KeysEnum, storageService } from 'teleport/services/storageService';
import * as service from 'teleport/services/userPreferences';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

export interface UserContextValue {
  preferences: UserPreferences;
  startPollingForOktaAccessLists(): void;
  updateDiscoverResourcePreferences: (
    preferences: Partial<DiscoverResourcePreference>
  ) => Promise<void>;
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

let startedPolling = false;
export function UserContextProvider(props: PropsWithChildren<unknown>) {
  const location = useLocation();
  const { attempt, run } = useAttempt('processing');
  // because we have to update cluster preferences with itself during the update
  // we useRef here to prevent infinite rerenders
  const clusterPreferences = useRef<Record<string, ClusterUserPreferences>>({});

  const [preferences, setPreferences] = useState<UserPreferences>(
    makeDefaultUserPreferences()
  );

  const toastNotification = useToastNotifications();

  const getClusterPinnedResources = useCallback(async (clusterId: string) => {
    if (clusterPreferences.current[clusterId]) {
      return clusterPreferences.current[clusterId].pinnedResources.resourceIds;
    }
    const prefs = await service.getUserClusterPreferences(clusterId);
    if (prefs) {
      clusterPreferences.current[clusterId] = prefs;
      return prefs.pinnedResources.resourceIds;
    }
    return null;
  }, []);

  const updateClusterPinnedResources = async (
    clusterId: string,
    pinnedResources: string[]
  ) => {
    if (!clusterPreferences.current[clusterId]) {
      clusterPreferences.current[clusterId] = {
        pinnedResources: { resourceIds: [] },
      };
    }
    clusterPreferences.current[clusterId].pinnedResources.resourceIds =
      pinnedResources;

    return service.updateUserClusterPreferences(clusterId, {
      ...preferences,
      clusterPreferences: clusterPreferences.current[clusterId],
    });
  };

  const updateDiscoverResourcePreferences = async (
    discoverResourcePreferences: Partial<DiscoverResourcePreference>
  ) => {
    const nextPreferences: UserPreferences = {
      ...preferences,
      ...discoverResourcePreferences,
    };

    return service.updateUserPreferences(nextPreferences).then(() => {
      setPreferences(nextPreferences);
      storageService.setUserPreferences(nextPreferences);
    });
  };

  async function loadUserPreferences() {
    const storedPreferences = storageService.getUserPreferences();

    try {
      const preferences = await service.getUserPreferences();
      clusterPreferences.current[cfg.proxyCluster] =
        preferences.clusterPreferences;

      setPreferences(preferences);
      storageService.setUserPreferences(preferences);
    } catch {
      if (storedPreferences) {
        setPreferences(storedPreferences);

        return;
      }
    }
  }

  async function findOktaAccessLists() {
    return accessManagementService
      .fetchAccessListsV2({
        // sort by name here. If they want to find a specific list to add, they will most
        // likely type, but this allows this form to work without extra steps to check
        // if the cache is healthy or not
        sort: { dir: 'ASC', fieldName: 'name' },
        search: 'okta',
        limit: 100,
      })
      .then(resp => {
        if (resp.agents?.length) {
          // TODO: pagination does not search through labels
          return resp.agents.some(al => al.origin === AccessListOrigin.Okta);
        } else {
          return false;
        }
      });
  }

  function startPollingForOktaAccessLists() {
    startedPolling = true;
    console.log('---- GOING TO START POLLING!!!!!!');
    // Set the interval to run myFunction every 2 seconds (2000 milliseconds)
    const intervalId = setInterval(() => {
      console.log('--- POLLING');
      findOktaAccessLists().then(hasOkta => {
        if (hasOkta) {
          startedPolling = false;
          clearInterval(intervalId);
          console.log(
            '--- window.location.pathname: ',
            window.location.pathname,
            ecfg.getNewAccessListRoute()
          );
          if (window.location.pathname !== ecfg.getNewAccessListRoute()) {
            toastNotification.add({
              severity: 'info',
              content: {
                title: `Finished syncing Okta Apps and User Groups as Access Lists`,
                description: `Set up Access for your Okta user groups`,
                icon: ShieldCheck,
                isAutoRemovable: false,
                action: {
                  content: 'Set Up Access',
                  internalLink: ecfg.getNewAccessListRoute(),
                },
              },
            });
          }
        }
      });
    }, 2000); // every 1 min
  }

  const existingPlugin = useFetchPlugin<'okta'>('okta');

  useEffect(() => {
    console.log(
      '--- i am here in user context ----',
      location.pathname === ecfg.getNewAccessListRoute()
    );
    if (!existingPlugin.isSuccess) {
      // okta plugin not found
      return;
    }

    const plugin = existingPlugin.data;
    const appGroupSyncEnabled =
      plugin.spec.enableAccessListSync ||
      plugin.status?.details?.accessListsSyncDetails?.enabled;
    if (appGroupSyncEnabled && !startedPolling) {
      findOktaAccessLists().then(hasOkta => {
        if (!hasOkta) {
          startPollingForOktaAccessLists();
        }
      });
    }
    // Once on init
  }, [existingPlugin.data, existingPlugin.isSuccess]);

  function updatePreferences(newPreferences: Partial<UserPreferences>) {
    const nextPreferences = {
      ...preferences,
      ...newPreferences,
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
      accessGraph: {
        ...preferences.accessGraph,
        ...newPreferences.accessGraph,
      },
    } as UserPreferences;

    setPreferences(nextPreferences);
    storageService.setUserPreferences(nextPreferences);

    return service.updateUserPreferences(nextPreferences);
  }

  useEffect(() => {
    function receiveMessage(event: StorageEvent) {
      if (!event.newValue || event.key !== KeysEnum.USER_PREFERENCES) {
        return;
      }

      setPreferences(JSON.parse(event.newValue));
    }

    storageService.subscribe(receiveMessage);

    return () => storageService.unsubscribe(receiveMessage);
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
        updateDiscoverResourcePreferences,
        startPollingForOktaAccessLists,
      }}
    >
      {props.children}
    </UserContext.Provider>
  );
}
