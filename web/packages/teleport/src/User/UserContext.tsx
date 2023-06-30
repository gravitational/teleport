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
  useEffect,
  useState,
} from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { Indicator } from 'design';

import * as service from 'teleport/services/userPreferences';

import storage, { KeysEnum } from 'teleport/services/localStorage';

import { ThemePreference } from 'teleport/services/userPreferences/types';

import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';

import { LoadingContainer } from 'teleport/User/Loading';

import type {
  UserPreferences,
  UserPreferencesSubset,
} from 'teleport/services/userPreferences/types';

interface UserContextValue {
  preferences: UserPreferences;
  updatePreferences: (preferences: UserPreferencesSubset) => Promise<void>;
}

const UserContext = createContext<UserContextValue>(null);

export function useUser() {
  return useContext(UserContext);
}

export function UserContextProvider(props: PropsWithChildren<unknown>) {
  const { attempt, run } = useAttempt('processing');

  const [preferences, setPreferences] = useState<UserPreferences>(
    makeDefaultUserPreferences()
  );

  async function loadUserPreferences() {
    const storedPreferences = storage.getUserPreferences();
    const theme = storage.getThemePreference();

    try {
      const preferences = await service.getUserPreferences();

      if (!storedPreferences) {
        // there are no mirrored user preferences in local storage so this is the first time
        // the user has requested their preferences

        // update the preferences with the previous theme setting in local storage
        preferences.theme = theme;

        if (theme !== ThemePreference.Light) {
          // the light theme is the default, so only update the backend if it is not light
          updatePreferences({ theme });
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
        setPreferences({ ...preferences, theme });
      }
    }
  }

  function updatePreferences(newPreferences: UserPreferencesSubset) {
    const nextPreferences = {
      ...preferences,
      ...newPreferences,
      assist: {
        ...preferences.assist,
        ...newPreferences.assist,
      },
    } as UserPreferences;

    setPreferences(nextPreferences);
    storage.setUserPreferences(nextPreferences);

    return service.updateUserPreferences(newPreferences);
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
      <LoadingContainer>
        <Indicator />
      </LoadingContainer>
    );
  }

  return (
    <UserContext.Provider value={{ preferences, updatePreferences }}>
      {props.children}
    </UserContext.Provider>
  );
}
