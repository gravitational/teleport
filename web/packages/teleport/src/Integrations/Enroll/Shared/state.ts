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
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react';
import { useLocation, useNavigate } from 'react-router';

import { SortOrder } from 'shared/components/Controls/SortMenu';

import { isIntegrationTag, type IntegrationTag } from './common';

export interface IntegrationPickerFilters {
  tags: IntegrationTag[];
}

export interface IntegrationPickerState {
  filters: IntegrationPickerFilters;
  search: string;
  sortKey: string | undefined;
  sortDirection: SortOrder;
}

export type IntegrationPickerFilterKey = keyof IntegrationPickerFilters;

export function searchParamsToState(params: URLSearchParams) {
  const state: IntegrationPickerState = {
    filters: {
      tags: [],
    },
    search: '',
    sortKey: undefined,
    sortDirection: 'ASC',
  };

  const sortKey = params.get('sort');
  if (sortKey && ['name'].includes(sortKey)) {
    state.sortKey = sortKey;
  }

  const direction = params.get('direction');
  if (direction === 'ASC' || direction === 'DESC') {
    state.sortDirection = direction;
  }

  const tags = params.getAll('tags');
  if (tags.length > 0) {
    state.filters.tags = tags.filter(isIntegrationTag);
  }

  const search = params.get('search');
  if (search) {
    state.search = search;
  }

  return state;
}

export function stateToSearchParams(state: IntegrationPickerState) {
  const urlParams = new URLSearchParams();

  if (state.filters.tags.length > 0) {
    for (const tags of state.filters.tags) {
      urlParams.append('tags', tags);
    }
  }

  if (state.search) {
    urlParams.set('search', state.search);
  }

  if (state.sortKey !== undefined) {
    urlParams.set('direction', state.sortDirection);
    urlParams.set('sort', state.sortKey);
  }

  return urlParams.toString();
}

// useIntegrationPickerState is a custom hook that manages the state of the the Integration Picker,
// syncing it with the URL search parameters.
// It allows for state updates and ensures that the URL reflects the current state.
// It also listens for changes in the URL to update the state accordingly.
// Repurposed from SessionRecordings/list/state.ts
export function useIntegrationPickerState(): [
  IntegrationPickerState,
  Dispatch<SetStateAction<IntegrationPickerState>>,
] {
  const location = useLocation();
  const navigate = useNavigate();

  const [state, setState] = useState<IntegrationPickerState>(() =>
    searchParamsToState(new URLSearchParams(location.search))
  );

  const currentSearch = useRef<string>(location.search);

  useEffect(() => {
    const params = stateToSearchParams(state);

    currentSearch.current = `?${params.toString()}`;

    if (
      location.search.length === 0 &&
      currentSearch.current.length === 1 // empty, i.e. just '?'
    ) {
      // the current search is empty, and the state is also empty,
      // so we don't need to update the URL
      return;
    }

    if (location.search !== currentSearch.current) {
      navigate({ search: currentSearch.current }, { replace: true });
    }
  }, [location.search, navigate, state]);

  // Listen for URL changes (e.g., browser back/forward navigation)
  useEffect(() => {
    if (location.search !== currentSearch.current) {
      setState(searchParamsToState(new URLSearchParams(location.search)));
      currentSearch.current = location.search;
    }
  }, [location.search]);

  return [state, setState];
}
