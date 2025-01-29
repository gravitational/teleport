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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { UnifiedResourcePreferences } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import {
  Attempt,
  CanceledError,
  hasFinished,
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
  mapAttempt,
  useAsync,
} from 'shared/hooks/useAsync';

import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';
import { UserPreferences } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri, routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function useUserPreferences(clusterUri: ClusterUri): {
  userPreferencesAttempt: Attempt<void>;
  updateUserPreferences(newPreferences: UserPreferences): Promise<void>;
  userPreferences: UserPreferences;
} {
  const appContext = useAppContext();
  const initialFetchAttemptAbortController = useRef(new AbortController());
  // Consider storing the unified resource view preferences on the document.
  // https://github.com/gravitational/teleport/pull/35251#discussion_r1424116275
  const [unifiedResourcePreferences, setUnifiedResourcePreferences] = useState<
    UserPreferences['unifiedResourcePreferences']
  >(
    appContext.workspacesService.getUnifiedResourcePreferences(
      routing.ensureRootClusterUri(clusterUri)
    )
  );
  const [clusterPreferences, setClusterPreferences] = useState<
    UserPreferences['clusterPreferences']
  >({
    // we pass an empty array, so pinning is enabled by default
    pinnedResources: { resourceIds: [] },
  });

  const [initialFetchAttempt, runInitialFetchAttempt] = useAsync(
    useCallback(
      async () =>
        retryWithRelogin(appContext, clusterUri, async () => {
          const { response } = await appContext.tshd.getUserPreferences(
            { clusterUri },
            {
              abort: cloneAbortSignal(
                initialFetchAttemptAbortController.current.signal
              ),
            }
          );
          return response.userPreferences;
        }),
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
    useCallback(
      async (newPreferences: UserPreferences) =>
        retryWithRelogin(appContext, clusterUri, async () => {
          const { response } = await appContext.tshd.updateUserPreferences({
            clusterUri,
            userPreferences: newPreferences,
          });
          return response.userPreferences;
        }),
      [appContext, clusterUri]
    )
  );

  const updateUnifiedResourcePreferencesStateAndWorkspace = useCallback(
    (unifiedResourcePreferences: UnifiedResourcePreferences) => {
      setUnifiedResourcePreferences(unifiedResourcePreferences);
      appContext.workspacesService.setUnifiedResourcePreferences(
        routing.ensureRootClusterUri(clusterUri),
        unifiedResourcePreferences
      );
    },
    [appContext.workspacesService, clusterUri]
  );

  useEffect(() => {
    const fetchPreferences = async () => {
      if (
        initialFetchAttempt.status === '' &&
        supersededInitialFetchAttempt.status === ''
      ) {
        const [prefs, error] = await runInitialFetchAttempt();
        if (!error) {
          updateUnifiedResourcePreferencesStateAndWorkspace(
            prefs.unifiedResourcePreferences
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
    initialFetchAttempt.status,
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
        // We always try to update cluster preferences based on the cluster response so that the
        // pinned resources are up-to-date.
        // We don't do it for unified resources preferences because if the view mode got updated on
        // the server while the user, say, updated a pin, we don't want to suddenly change the view
        // mode.
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
    userPreferencesAttempt: useMemo(
      () =>
        supersededInitialFetchAttempt.status !== ''
          ? supersededInitialFetchAttempt
          : mapAttempt(initialFetchAttempt, () => undefined),
      [initialFetchAttempt, supersededInitialFetchAttempt]
    ),
    updateUserPreferences,
    userPreferences: useMemo(
      () => ({
        unifiedResourcePreferences,
        clusterPreferences,
      }),
      [clusterPreferences, unifiedResourcePreferences]
    ),
  };
}
