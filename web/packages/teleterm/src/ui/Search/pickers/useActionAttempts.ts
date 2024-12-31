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
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
} from 'react';

import { makeEmptyAttempt, mapAttempt, useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { mapToAction } from 'teleterm/ui/Search/actions';
import { useSearchContext } from 'teleterm/ui/Search/SearchContext';
import { SearchFilter } from 'teleterm/ui/Search/searchResult';
import {
  rankResults,
  useFilterSearch,
  useResourceSearch,
} from 'teleterm/ui/Search/useSearch';
import { routing } from 'teleterm/ui/uri';
import { isRetryable } from 'teleterm/ui/utils/retryWithRelogin';
import { useVnetContext, useVnetLauncher } from 'teleterm/ui/Vnet';

import { useDisplayResults } from './useDisplayResults';

export function useActionAttempts() {
  const ctx = useAppContext();
  const { modalsService, workspacesService } = ctx;
  const searchContext = useSearchContext();
  const { inputValue, filters, pauseUserInteraction } = searchContext;
  const { isSupported: isVnetSupported } = useVnetContext();
  const vnetLauncher = useVnetLauncher();
  const launchVnet = isVnetSupported ? vnetLauncher : undefined;

  const [resourceSearchAttempt, runResourceSearch, setResourceSearchAttempt] =
    useAsync(useResourceSearch());
  /**
   * runRetryableResourceSearch implements retryWithRelogin logic for resource search. We check if
   * among all resource search requests there's a request belonging to the current workspace and if
   * it failed with a retryable error. If so, we show the login modal.
   *
   * We're interested only in errors coming from clusters within the current workspace because
   * otherwise we'd nag the user to log in to each cluster they've added to the app.
   *
   * Technically we could wrap runResourceSearch into a custom promise which would make it fit into
   * retryWithRelogin. However, by doing this we'd give up some of the finer control over the whole
   * process, for example we couldn't as easily call pauseUserInteraction only when necessary.
   *
   * This logic _cannot_ be included within the callback passed to useAsync and has to be performed
   * outside of it. If it was included within useAsync, then this logic would fire for any request
   * made to the cluster, even for stale requests which are ignored by useAsync.
   */
  const runRetryableResourceSearch = useCallback(
    async (search: string, filters: SearchFilter[]): Promise<void> => {
      const activeRootClusterUri = workspacesService.getRootClusterUri();
      const [results, err] = await runResourceSearch(
        search,
        filters,
        searchContext.advancedSearchEnabled
      );
      // Since resource search uses Promise.allSettled underneath, the only error that could be
      // returned here is CanceledError from useAsync. In that case, we can just return early.
      if (err) {
        return;
      }

      const hasActiveWorkspaceRequestFailedWithRetryableError =
        results.errors.some(
          error =>
            routing.belongsToProfile(activeRootClusterUri, error.clusterUri) &&
            isRetryable(error.cause)
        );

      if (!hasActiveWorkspaceRequestFailedWithRetryableError) {
        return;
      }

      await pauseUserInteraction(
        () =>
          new Promise<void>(resolve => {
            modalsService.openRegularDialog({
              kind: 'cluster-connect',
              clusterUri: activeRootClusterUri,
              reason: undefined,
              prefill: undefined,
              onSuccess: () => resolve(),
              onCancel: () => resolve(),
            });
          })
      );

      // Retrying the request no matter if the user logged in through the modal or not, for the same
      // reasons as described in retryWithRelogin.
      runResourceSearch(search, filters, searchContext.advancedSearchEnabled);
    },
    [
      searchContext.advancedSearchEnabled,
      workspacesService,
      runResourceSearch,
      pauseUserInteraction,
      modalsService,
    ]
  );
  const runDebouncedResourceSearch = useDebounce(
    runRetryableResourceSearch,
    200
  );
  const resourceActionsAttempt = useMemo(
    () =>
      mapAttempt(resourceSearchAttempt, ({ results, search }) =>
        rankResults(results, search).map(result =>
          mapToAction(ctx, launchVnet, searchContext, result)
        )
      ),
    [ctx, resourceSearchAttempt, searchContext, launchVnet]
  );

  const runFilterSearch = useFilterSearch();
  const filterActions = useMemo(
    () =>
      // TODO(gzdunek): filters are sorted inline, should be done here to align with resource search
      runFilterSearch(inputValue, filters).map(result =>
        mapToAction(ctx, launchVnet, searchContext, result)
      ),
    [runFilterSearch, inputValue, filters, ctx, searchContext, launchVnet]
  );

  const displayResultsAction = mapToAction(
    ctx,
    launchVnet,
    searchContext,
    useDisplayResults({
      inputValue,
      filters,
    })
  );

  useEffect(() => {
    // Reset the resource search attempt as soon as the input changes. If we didn't do that, then
    // the attempt would only get updated on debounce. This could lead to the following scenario:
    //
    // 1. You type in `foo`, wait for the results to show up.
    // 2. You clear the input and quickly type in `bar`.
    // 3. Now you see the stale results for `foo`, because the debounce didn't kick in yet.
    setResourceSearchAttempt(makeEmptyAttempt());

    runDebouncedResourceSearch(inputValue, filters);
  }, [
    inputValue,
    filters,
    setResourceSearchAttempt,
    runFilterSearch,
    runDebouncedResourceSearch,
    searchContext.advancedSearchEnabled,
  ]);

  return {
    displayResultsAction,
    filterActions,
    resourceActionsAttempt,
    /**
     * resourceSearchAttempt is the raw version of useResourceSearch attempt that has not been
     * mapped to actions. Returning this will allow ActionPicker to inspect errors returned from the
     * resource search.
     *
     * The status of this attempt never equals to 'error'.
     * */
    resourceSearchAttempt,
  };
}

function useDebounce<Args extends unknown[], ReturnValue>(
  callback: (...args: Args) => ReturnValue,
  delay: number
) {
  const callbackRef = useRef(callback);
  useLayoutEffect(() => {
    callbackRef.current = callback;
  });
  return useMemo(
    () => debounce((...args: Args) => callbackRef.current(...args), delay),
    [delay]
  );
}
