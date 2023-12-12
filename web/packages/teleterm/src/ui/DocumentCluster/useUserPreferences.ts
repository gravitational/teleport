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

import { useCallback, useEffect, useRef, useState } from 'react';

import {
  useAsync,
  Attempt,
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
  makeSuccessAttempt,
  mapAttempt,
  CanceledError,
  hasFinished,
} from 'shared/hooks/useAsync';

import {
  DefaultTab,
  ViewMode,
  UnifiedResourcePreferences,
} from 'shared/services/unifiedResourcePreferences';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { routing, ClusterUri } from 'teleterm/ui/uri';

import { UserPreferences } from 'teleterm/services/tshd/types';
import { retryWithRelogin } from 'teleterm/ui/utils';
import createAbortController from 'teleterm/services/tshd/createAbortController';

export function useUserPreferences(clusterUri: ClusterUri): {
  userPreferencesAttempt: Attempt<void>;
  updateUserPreferences(newPreferences: UserPreferences): Promise<void>;
  userPreferences: UserPreferences;
} {
  const appContext = useAppContext();
  const initialFetchAttemptAbortController = useRef(createAbortController());
  const [unifiedResourcePreferences, setUnifiedResourcePreferences] = useState<
    UserPreferences['unifiedResourcePreferences']
  >(
    mergeWithDefaultUnifiedResourcePreferences(
      appContext.workspacesService.getUnifiedResourcePreferences(
        routing.ensureRootClusterUri(clusterUri)
      )
    ) || {
      defaultTab: DefaultTab.DEFAULT_TAB_ALL,
      viewMode: ViewMode.VIEW_MODE_CARD,
    }
  );
  const [clusterPreferences, setClusterPreferences] = useState<
    UserPreferences['clusterPreferences']
  >({
    // we pass an empty array, so pinning is enabled by default
    pinnedResources: { resourceIdsList: [] },
  });

  const [initialFetchAttempt, runInitialFetchAttempt] = useAsync(
    useCallback(
      async () =>
        retryWithRelogin(appContext, clusterUri, () =>
          appContext.tshd.getUserPreferences(
            { clusterUri },
            initialFetchAttemptAbortController.current.signal
          )
        ),
      [appContext, clusterUri]
    )
  );

  // In a situation where the initial fetch attempt is still in progress,
  // but the user has changed the preferences, we want
  // to abort the previous attempt and replace it with the update attempt.
  // This is done through `supersededInitialFetchAttempt`.
  const [supersededInitialFetchAttempt, setSupersededInitialFetchAttempt] =
    useState<Attempt<void>>(makeEmptyAttempt());

  const [, runUpdateAttempt] = useAsync(
    async (newPreferences: UserPreferences) =>
      retryWithRelogin(appContext, clusterUri, () =>
        appContext.tshd.updateUserPreferences({
          clusterUri,
          userPreferences: newPreferences,
        })
      )
  );

  const updateUnifiedResourcePreferencesStateAndWorkspace = useCallback(
    (unifiedResourcePreferences: UnifiedResourcePreferences) => {
      const prefsWithDefaults = mergeWithDefaultUnifiedResourcePreferences(
        unifiedResourcePreferences
      );
      setUnifiedResourcePreferences(prefsWithDefaults);
      appContext.workspacesService.setUnifiedResourcePreferences(
        routing.ensureRootClusterUri(clusterUri),
        prefsWithDefaults
      );
    },
    [appContext.workspacesService, clusterUri]
  );

  useEffect(() => {
    const fetchPreferences = async () => {
      if (supersededInitialFetchAttempt.status === '') {
        const [prefs, error] = await runInitialFetchAttempt();
        if (!error) {
          updateUnifiedResourcePreferencesStateAndWorkspace(
            prefs?.unifiedResourcePreferences
          );
          setClusterPreferences(prefs?.clusterPreferences);
        }
      }
    };

    fetchPreferences();
  }, [
    supersededInitialFetchAttempt.status,
    runInitialFetchAttempt,
    updateUnifiedResourcePreferencesStateAndWorkspace,
  ]);

  const hasUpdateSupersededInitialFetch =
    initialFetchAttempt.status !== 'success' &&
    !hasFinished(supersededInitialFetchAttempt);
  const updateUserPreferences = useCallback(
    async (newPreferences: Partial<UserPreferences>): Promise<void> => {
      if (newPreferences.unifiedResourcePreferences) {
        updateUnifiedResourcePreferencesStateAndWorkspace(
          newPreferences.unifiedResourcePreferences
        );
      }

      if (hasUpdateSupersededInitialFetch) {
        setSupersededInitialFetchAttempt(makeProcessingAttempt());
        initialFetchAttemptAbortController.current.abort();
      }

      const [prefs, error] = await runUpdateAttempt(newPreferences);
      if (!error) {
        // We always try to update cluster preferences,
        // so the user sees the recent pinned resources.
        // We don't do it for unified resources preferences
        // because we don't want to suddenly change the view.
        setClusterPreferences(prefs?.clusterPreferences);
        if (hasUpdateSupersededInitialFetch) {
          setSupersededInitialFetchAttempt(makeSuccessAttempt(undefined));
        }
        return;
      }
      if (!(error instanceof CanceledError)) {
        if (hasUpdateSupersededInitialFetch) {
          setSupersededInitialFetchAttempt(makeErrorAttempt(error));
        }
        appContext.notificationsService.notifyWarning({
          title: 'Failed to update user preferences',
          description: error.message,
        });
      }
    },
    [
      hasUpdateSupersededInitialFetch,
      runUpdateAttempt,
      updateUnifiedResourcePreferencesStateAndWorkspace,
      appContext.notificationsService,
    ]
  );

  return {
    userPreferencesAttempt:
      supersededInitialFetchAttempt.status !== ''
        ? supersededInitialFetchAttempt
        : mapAttempt(initialFetchAttempt, () => undefined),
    updateUserPreferences,
    userPreferences: {
      unifiedResourcePreferences,
      clusterPreferences,
    },
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
