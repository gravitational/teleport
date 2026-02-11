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

import { useCallback, useMemo } from 'react';
import { useHistory, useLocation } from 'react-router';

export interface UsersUrlState {
  search: string;
  user: string | null;
}

export function searchParamsToState(params: URLSearchParams): UsersUrlState {
  const state: UsersUrlState = {
    search: params.get('search') ?? '',
    user: params.get('user') ?? null,
  };

  return state;
}

export function stateToSearchParams(state: UsersUrlState): string {
  const urlParams = new URLSearchParams();

  if (state.search) {
    urlParams.set('search', state.search);
  }

  if (state.user) {
    urlParams.set('user', state.user);
  }

  return urlParams.toString();
}

export function useUrlParams(): [
  UsersUrlState,
  (newState: Partial<UsersUrlState>) => void,
] {
  const history = useHistory();
  const location = useLocation();

  const params = useMemo(() => {
    return searchParamsToState(new URLSearchParams(location.search));
  }, [location.search]);

  const setParams = useCallback(
    (next: UsersUrlState) => {
      const current = searchParamsToState(new URLSearchParams(location.search));

      const hasChanged =
        current.search !== next.search || current.user !== next.user;

      if (hasChanged) {
        const nextParams = stateToSearchParams(next);
        const nextSearch = nextParams ? `?${nextParams}` : '';
        history.push({ search: nextSearch });
      }
    },
    [location.search, history]
  );

  return [params, setParams];
}
