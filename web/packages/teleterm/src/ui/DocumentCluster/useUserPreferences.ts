/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useCallback, useEffect, useRef } from 'react';

import {
  useAsync,
  makeSuccessAttempt,
  mapAttempt,
  Attempt,
} from 'shared/hooks/useAsync';

import {
  DefaultTab,
  ViewMode,
  UnifiedResourcePreferences,
} from 'shared/services/unifiedResourcePreferences';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { routing, ClusterUri } from 'teleterm/ui/uri';

import { UserPreferences } from 'teleterm/services/tshd/types';

export function useUserPreferences(clusterUri: ClusterUri): {
  userPreferencesAttempt: Attempt<UserPreferences>;
  unifiedResourcePreferencesFallback: UnifiedResourcePreferences;
  updateUserPreferencesAttempt: Attempt<void>;
  updateUserPreferences(newPreferences: UserPreferences): Promise<void>;
} {
  const appContext = useAppContext();
  const [
    fetchUserPreferencesAttempt,
    runFetchUserPreferencesAttempt,
    setFetchUserPreferencesAttempt,
  ] = useAsync<Promise<UserPreferences>[], UserPreferences>(value => value);
  const [updateUserPreferencesAttempt, runUpdateUserPreferencesAttempt] =
    useAsync(async (newPreferences: UserPreferences) =>
      appContext.tshd.updateUserPreferences({
        clusterUri,
        userPreferences: newPreferences,
      })
    );
  appContext.workspacesService.useState();

  const getPreferencesPromise = useRef<ReturnType<typeof getPreferences>>();
  const getPreferences = useCallback(async () => {
    const preferencesPromise = appContext.tshd.getUserPreferences({
      clusterUri,
    });
    getPreferencesPromise.current = preferencesPromise;
    return preferencesPromise;
  }, [appContext.tshd, clusterUri]);

  const unifiedResourcePreferencesFallback =
    appContext.workspacesService.getUnifiedResourcePreferences(
      routing.ensureRootClusterUri(clusterUri)
    ) || {
      defaultTab: DefaultTab.DEFAULT_TAB_ALL,
      viewMode: ViewMode.VIEW_MODE_CARD,
    };

  const updateFetchAttemptAndWorkspace = useCallback(
    async (preferencesPromise: Promise<UserPreferences>) => {
      const [updated, error] = await runFetchUserPreferencesAttempt(
        preferencesPromise
      );
      if (!error) {
        appContext.workspacesService.setUnifiedResourcePreferences(
          routing.ensureRootClusterUri(clusterUri),
          updated.unifiedResourcePreferences
        );
      }
    },
    [appContext.workspacesService, clusterUri, runFetchUserPreferencesAttempt]
  );

  useEffect(() => {
    if (fetchUserPreferencesAttempt.status === '') {
      updateFetchAttemptAndWorkspace(getPreferences());
    }
  }, [
    updateFetchAttemptAndWorkspace,
    fetchUserPreferencesAttempt.status,
    getPreferences,
  ]);

  async function updateUserPreferences(
    newPreferences: Partial<UserPreferences>
  ): Promise<void> {
    // If we don't have the preferences fetched yet,
    // we start from updating the workspace (so the user sees an update in the UI).
    //
    // Since we know that the preferences have changed,
    // we don't want to show the user the preferences from the "get"
    // request that are outdated.
    // Instead, we replace a request in that attempt with an "update" request,
    // falling back to "get" response, only if the update failed.
    // Thanks to that, the user sees a continuous loading screen.
    if (fetchUserPreferencesAttempt.status !== 'success') {
      if (newPreferences.unifiedResourcePreferences) {
        appContext.workspacesService.setUnifiedResourcePreferences(
          routing.ensureRootClusterUri(clusterUri),
          newPreferences.unifiedResourcePreferences
        );
      }

      const updater = async (): Promise<UserPreferences> => {
        const [updated, error] = await runUpdateUserPreferencesAttempt(
          newPreferences
        );
        if (error) {
          return getPreferencesPromise.current;
        }
        return updated;
      };

      await updateFetchAttemptAndWorkspace(updater());
    } else {
      // Update the local state first, so the user sees an update in the UI.
      setFetchUserPreferencesAttempt(prevState =>
        makeSuccessAttempt({ ...prevState.data, ...newPreferences })
      );
      const [updated, error] = await runUpdateUserPreferencesAttempt(
        newPreferences
      );
      if (!error) {
        appContext.workspacesService.setUnifiedResourcePreferences(
          routing.ensureRootClusterUri(clusterUri),
          updated.unifiedResourcePreferences
        );
        setFetchUserPreferencesAttempt(makeSuccessAttempt(updated));
      }
    }
  }

  return {
    userPreferencesAttempt: mapAttempt(
      fetchUserPreferencesAttempt,
      attemptData => ({
        ...attemptData,
        unifiedResourcePreferences: mergeWithDefaultUnifiedResourcePreferences(
          attemptData.unifiedResourcePreferences
        ),
      })
    ),
    unifiedResourcePreferencesFallback:
      mergeWithDefaultUnifiedResourcePreferences(
        unifiedResourcePreferencesFallback
      ),
    updateUserPreferencesAttempt: mapAttempt(
      updateUserPreferencesAttempt,
      () => undefined
    ),
    updateUserPreferences,
  };
}

// TODO(gzdunek): DELETE IN 16.0.0.
// Support for UnifiedTabPreference has been added in 14.1 and for
// UnifiedViewModePreference in 14.1.5.
// We have to support these values being undefined/unset in Connect v15.
function mergeWithDefaultUnifiedResourcePreferences(
  unifiedResourcePreferences: UnifiedResourcePreferences
): UnifiedResourcePreferences {
  return {
    defaultTab: unifiedResourcePreferences
      ? unifiedResourcePreferences.defaultTab
      : DefaultTab.DEFAULT_TAB_ALL,
    viewMode:
      unifiedResourcePreferences &&
      unifiedResourcePreferences.viewMode !== ViewMode.VIEW_MODE_UNSPECIFIED
        ? unifiedResourcePreferences.viewMode
        : ViewMode.VIEW_MODE_CARD,
  };
}
