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

import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useCallback,
} from 'react';
import {
  makeEmptyAttempt,
  makeSuccessAttempt,
  mapAttempt,
  useAsync,
} from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import {
  rankResults,
  useFilterSearch,
  useResourceSearch,
} from 'teleterm/ui/Search/useSearch';
import { mapToActions } from 'teleterm/ui/Search/actions';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useSearchContext } from 'teleterm/ui/Search/SearchContext';
import { SearchFilter } from 'teleterm/ui/Search/searchResult';
import { routing } from 'teleterm/ui/uri';
import { isRetryable } from 'teleterm/ui/utils/retryWithRelogin';

export function useActionAttempts() {
  const ctx = useAppContext();
  const { modalsService, workspacesService } = ctx;
  const searchContext = useSearchContext();
  const { inputValue, filters, pauseUserInteraction } = searchContext;

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
      const [results, err] = await runResourceSearch(search, filters);
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
            modalsService.openClusterConnectDialog({
              clusterUri: activeRootClusterUri,
              onSuccess: () => resolve(),
              onCancel: () => resolve(),
            });
          })
      );

      // Retrying the request no matter if the user logged in through the modal or not, for the same
      // reasons as described in retryWithRelogin.
      runResourceSearch(search, filters);
    },
    [modalsService, workspacesService, runResourceSearch, pauseUserInteraction]
  );
  const runDebouncedResourceSearch = useDebounce(
    runRetryableResourceSearch,
    200
  );
  const resourceActionsAttempt = useMemo(
    () =>
      mapAttempt(resourceSearchAttempt, ({ results, search }) => {
        const sortedResults = rankResults(results, search);

        return mapToActions(ctx, searchContext, sortedResults);
      }),
    [ctx, resourceSearchAttempt, searchContext]
  );

  const runFilterSearch = useFilterSearch();
  const filterActionsAttempt = useMemo(() => {
    // TODO(gzdunek): filters are sorted inline, should be done here to align with resource search
    const filterSearchResults = runFilterSearch(inputValue, filters);
    const filterActions = mapToActions(ctx, searchContext, filterSearchResults);

    return makeSuccessAttempt(filterActions);
  }, [runFilterSearch, inputValue, filters, ctx, searchContext]);

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
  ]);

  return {
    filterActionsAttempt,
    resourceActionsAttempt,
    // resourceSearchAttempt is the raw version of useResourceSearch attempt that has not been
    // mapped to actions. Returning this will allow ActionPicker to inspect errors returned from the
    // resource search.
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
