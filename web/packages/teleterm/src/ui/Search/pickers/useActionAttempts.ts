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

import { useEffect, useLayoutEffect, useMemo, useRef } from 'react';
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
import Logger from 'teleterm/logger';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useSearchContext } from 'teleterm/ui/Search/SearchContext';

export function useActionAttempts() {
  const searchLogger = useRef(new Logger('search'));
  const ctx = useAppContext();
  // Both states are used by mapToActions.
  ctx.workspacesService.useState();
  ctx.clustersService.useState();
  const searchContext = useSearchContext();
  const { inputValue, filters } = searchContext;

  const [resourceSearchAttempt, runResourceSearch, setResourceSearchAttempt] =
    useAsync(useResourceSearch());
  const runResourceSearchDebounced = useDebounce(runResourceSearch, 200);
  const resourceActionsAttempt = useMemo(
    () =>
      mapAttempt(resourceSearchAttempt, ({ results, search }) => {
        const sortedResults = rankResults(results, search);
        searchLogger.current.info('results for', search, sortedResults);

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

    runResourceSearchDebounced(inputValue, filters);
  }, [
    inputValue,
    filters,
    setResourceSearchAttempt,
    runFilterSearch,
    runResourceSearchDebounced,
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
