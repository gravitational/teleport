/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
  startTransition,
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react';
import { useHistory } from 'react-router';

import {
  EventRange,
  getRangeOptions,
} from 'teleport/components/EventRangePicker';
import {
  validateRecordingType,
  type RecordingType,
} from 'teleport/services/recordings';

const sortKeys = ['date', 'type'] as const;

export type RecordingsListSortKey = (typeof sortKeys)[number];
export type RecordingsListSortDirection = 'ASC' | 'DESC';

function isValidSortKey(key: string): key is RecordingsListSortKey {
  return sortKeys.includes(key as RecordingsListSortKey);
}

export interface RecordingsListFilters {
  hideNonInteractive: boolean;
  resources: string[];
  types: RecordingType[];
  users: string[];
}

export interface RecordingsListState {
  filters: RecordingsListFilters;
  page: number;
  range: EventRange;
  search: string;
  sortKey: RecordingsListSortKey;
  sortDirection: RecordingsListSortDirection;
}

export type RecordingsListFilterKey = keyof RecordingsListFilters;

export function searchParamsToState(
  ranges: EventRange[],
  params: URLSearchParams
) {
  const state: RecordingsListState = {
    filters: {
      hideNonInteractive: false,
      types: [],
      resources: [],
      users: [],
    },
    page: 0,
    range: ranges[0],
    search: '',
    sortKey: 'date',
    sortDirection: 'DESC',
  };

  // if the user is using a predefined range, we grab it from the URL params
  // otherwise we use the custom range defined by the user
  const timeframe = params.get('timeframe');

  if (timeframe) {
    const timeframeNumber = parseInt(timeframe, 10);

    if (!isNaN(timeframeNumber) && timeframeNumber > 0) {
      const range = ranges[timeframeNumber];

      if (range) {
        state.range = range;
      }
    }
  } else {
    const from = params.get('from');
    const to = params.get('to');

    if (from && to) {
      const fromDate = new Date(from);
      const toDate = new Date(to);

      if (!isNaN(fromDate.getTime()) && !isNaN(toDate.getTime())) {
        state.range = {
          from: fromDate,
          to: toDate,
          isCustom: true,
        };
      }
    }
  }

  const resources = params.getAll('resources');
  if (resources.length > 0) {
    state.filters.resources = resources;
  }

  const types = params.getAll('types');
  if (types.length > 0) {
    state.filters.types = types.filter(validateRecordingType);
  }

  const users = params.getAll('users');
  if (users.length > 0) {
    state.filters.users = users;
  }

  const hideNonInteractive = params.get('hide_non_interactive');
  if (hideNonInteractive === 'true') {
    state.filters.hideNonInteractive = true;
  }

  const sortKey = params.get('sort');
  if (sortKey && isValidSortKey(sortKey)) {
    state.sortKey = sortKey as RecordingsListSortKey;
  }

  const direction = params.get('direction');
  if (direction === 'ASC' || direction === 'DESC') {
    state.sortDirection = direction as RecordingsListSortDirection;
  }

  const page = params.get('page');
  if (page) {
    const pageNumber = parseInt(page, 10);

    if (!isNaN(pageNumber) && pageNumber >= 0) {
      state.page = pageNumber;
    }
  }

  const search = params.get('search');
  if (search) {
    state.search = search;
  }

  return state;
}

export function stateToSearchParams(state: RecordingsListState) {
  const urlParams = new URLSearchParams();

  if (state.range.isCustom) {
    urlParams.set('from', state.range.from.toISOString());
    urlParams.set('to', state.range.to.toISOString());
  } else {
    const index = getRangeOptions().findIndex(r => r.name === state.range.name);

    // avoid setting timeframe for the default range
    if (index > 0) {
      urlParams.set('timeframe', index.toString());
    }
  }

  if (state.filters.resources.length > 0) {
    for (const resource of state.filters.resources) {
      urlParams.append('resources', resource);
    }
  }

  if (state.filters.types.length > 0) {
    for (const type of state.filters.types) {
      urlParams.append('types', type);
    }
  }

  if (state.filters.users.length > 0) {
    for (const user of state.filters.users) {
      urlParams.append('users', user);
    }
  }

  if (state.sortKey !== 'date') {
    urlParams.set('sort', state.sortKey);
  }

  if (state.sortDirection !== 'DESC') {
    urlParams.set('direction', state.sortDirection);
  }

  if (state.page > 0) {
    urlParams.set('page', state.page.toString());
  }

  if (state.filters.hideNonInteractive) {
    urlParams.set('hide_non_interactive', 'true');
  }

  if (state.search) {
    urlParams.set('search', state.search);
  }

  return urlParams.toString();
}

function sortArray(array: string[]) {
  return array.toSorted((a, b) => a.localeCompare(b));
}

function arraysAreEqual(a: string[], b: string[]) {
  if (a.length !== b.length) {
    return false;
  }

  return JSON.stringify(sortArray(a)) === JSON.stringify(sortArray(b));
}

function filtersAreEqual(a: RecordingsListFilters, b: RecordingsListFilters) {
  return (
    a.hideNonInteractive === b.hideNonInteractive &&
    arraysAreEqual(a.types, b.types) &&
    arraysAreEqual(a.users, b.users) &&
    arraysAreEqual(a.resources, b.resources)
  );
}

export function statesAreEqual(a: RecordingsListState, b: RecordingsListState) {
  return (
    a.range.from.getTime() === b.range.from.getTime() &&
    a.range.to.getTime() === b.range.to.getTime() &&
    a.range.isCustom === b.range.isCustom &&
    filtersAreEqual(a.filters, b.filters) &&
    a.sortKey === b.sortKey &&
    a.sortDirection === b.sortDirection &&
    a.page === b.page &&
    a.search === b.search
  );
}

// useRecordingsListState is a custom hook that manages the state of the recordings list,
// syncing it with the URL search parameters.
// It allows for state updates and ensures that the URL reflects the current state.
// It also listens for changes in the URL to update the state accordingly.
// (Provides a faster user experience vs. deriving the state directly from the URL)
export function useRecordingsListState(
  ranges: EventRange[]
): [RecordingsListState, Dispatch<SetStateAction<RecordingsListState>>] {
  const history = useHistory();

  const [state, _setState] = useState<RecordingsListState>(() =>
    searchParamsToState(ranges, new URLSearchParams(history.location.search))
  );

  const setState = useCallback(
    (action: SetStateAction<RecordingsListState>) => {
      startTransition(() => {
        _setState(prev => {
          let next: RecordingsListState;

          if (typeof action === 'function') {
            next = action(prev);

            if (statesAreEqual(prev, next)) {
              next = prev;
            }
          } else if (statesAreEqual(prev, action)) {
            next = prev;
          } else {
            next = action;
          }

          return next;
        });
      });
    },
    []
  );

  const currentSearch = useRef<string>(history.location.search);

  useEffect(() => {
    const params = stateToSearchParams(state);

    currentSearch.current = `?${params.toString()}`;

    if (
      history.location.search.length === 0 &&
      currentSearch.current.length === 1 // empty, i.e. just '?'
    ) {
      // the current search is empty, and the state is also empty,
      // so we don't need to update the URL
      return;
    }

    if (history.location.search !== currentSearch.current) {
      history.replace({
        hash: history.location.hash,
        search: currentSearch.current,
      });
    }
  }, [history, state]);

  useEffect(() => {
    return history.listen(next => {
      if (next.search !== currentSearch.current) {
        _setState(
          searchParamsToState(ranges, new URLSearchParams(next.search))
        );

        currentSearch.current = next.search;
      }
    });
  }, [history, ranges]);

  return [state, setState] as const;
}
