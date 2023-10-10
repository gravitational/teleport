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
  PropsWithChildren,
  useContext,
  useCallback,
  useEffect,
  useState,
  useRef,
} from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import {
  useAsync,
  Attempt,
  makeSuccessAttempt,
  makeEmptyAttempt,
} from 'shared/hooks/useAsync';

import { Indicator } from 'design';

import { StyledIndicator } from 'teleport/Main';

import * as service from 'teleport/services/userPreferences';

import storage, { KeysEnum } from 'teleport/services/localStorage';

import {
  deprecatedThemeToThemePreference,
  ThemePreference,
} from 'teleport/services/userPreferences/types';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import useStickyClusterId from 'teleport/useStickyClusterId';

import type {
  UserClusterPreferences,
  UserPreferences,
} from 'teleport/services/userPreferences/types';

export interface UserContextValue {
  preferences: UserPreferences;
  updatePreferences: (preferences: Partial<UserPreferences>) => Promise<void>;
  updateClusterPreferences: (
    preferences: Partial<UserClusterPreferences>
  ) => void;
  updateClusterPreferencesAttempt: Attempt<any>; // returns nothing
  clusterPreferencesAttempt: Attempt<UserClusterPreferences>;
}

export const UserContext = createContext<UserContextValue>(null);

export function useUser(): UserContextValue {
  return useContext(UserContext);
}

const isAbortError = (err: any): boolean =>
  (err instanceof DOMException && err.name === 'AbortError') ||
  (err.cause && isAbortError(err.cause));

export function UserContextProvider(props: PropsWithChildren<unknown>) {
  const { attempt, run } = useAttempt('processing');
  const { clusterId } = useStickyClusterId();
  const clusterAbortRef = useRef(new AbortController());

  const [
    clusterPreferencesAttempt,
    clusterPreferencesRun,
    setClusterPreferencesAttempt,
  ] = useAsync(
    useCallback((clusterId: string) => {
      try {
        return service.getUserClusterPreferences(
          clusterId,
          clusterAbortRef.current.signal
        );
      } catch (error) {
        if (isAbortError(error)) {
          // ignore CanceledError
          return;
        }
        // throw everything else
        throw error;
      }
    }, [])
  );

  const [
    updateClusterPreferencesAttempt,
    updateClusterPreferencesRun,
    setUpdateClusterPreferencesAttempt,
  ] = useAsync(
    useCallback(
      (clusterId: string, nextPreferences: Partial<UserPreferences>) => {
        return service.updateUserClusterPreferences(clusterId, nextPreferences);
      },
      []
    )
  );

  const [preferences, setPreferences] = useState<UserPreferences>(
    makeDefaultUserPreferences()
  );

  async function loadUserPreferences() {
    const storedPreferences = storage.getUserPreferences();
    const theme = storage.getDeprecatedThemePreference();

    try {
      const preferences = await service.getUserPreferences();
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
      clusterPreferences: clusterPreferencesAttempt.data,
    } as UserPreferences;

    setPreferences(nextPreferences);
    storage.setUserPreferences(nextPreferences);

    return service.updateUserPreferences(nextPreferences);
  }

  function updateClusterPreferences(
    newPreferences: Partial<UserClusterPreferences>
  ) {
    const nextPreferences = {
      ...clusterPreferencesAttempt.data,
      ...newPreferences,
    };

    setClusterPreferencesAttempt(makeSuccessAttempt(nextPreferences));
    updateClusterPreferencesRun(clusterId, {
      ...preferences,
      clusterPreferences: nextPreferences,
    });
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

  useEffect(() => {
    clusterPreferencesRun(clusterId);
    setUpdateClusterPreferencesAttempt(makeEmptyAttempt());
    const current = clusterAbortRef.current;
    return () => {
      current.abort();
    };
  }, [clusterId, clusterPreferencesRun, setUpdateClusterPreferencesAttempt]);

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
        clusterPreferencesAttempt,
        updateClusterPreferences,
        updateClusterPreferencesAttempt,
      }}
    >
      {props.children}
    </UserContext.Provider>
  );
}
