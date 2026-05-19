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

import { act, renderHook } from '@testing-library/react';
import { createMemoryHistory } from 'history';
import type { PropsWithChildren } from 'react';
import { Router } from 'react-router';

import {
  searchParamsToState,
  stateToSearchParams,
  useUrlParams,
  type UsersUrlState,
} from './state';

describe('searchParamsToState', () => {
  it('returns default state when no params provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(params);

    expect(state).toEqual({
      search: '',
      user: null,
    });
  });

  it('parses parameters', () => {
    const params = new URLSearchParams('?search=admin&user=bob');
    const state = searchParamsToState(params);

    expect(state.search).toBe('admin');
    expect(state.user).toBe('bob');
  });
});

describe('stateToSearchParams', () => {
  it('returns empty params for default state', () => {
    const state: UsersUrlState = {
      search: '',
      user: null,
    };

    const params = stateToSearchParams(state);
    expect(params).toBe('');
  });

  it('combines all parameters correctly', () => {
    const state: UsersUrlState = {
      search: 'test',
      user: 'alice@company.com',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('search')).toBe('test');
    expect(urlParams.get('user')).toBe('alice@company.com');
  });
});

describe('useUrlParams', () => {
  it('initializes params from URL search params', () => {
    const history = createMemoryHistory({
      initialEntries: ['/users?search=test&user=alice@company.com'],
    });

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useUrlParams(), {
      wrapper,
    });

    const [params] = result.current;

    expect(params.search).toBe('test');
    expect(params.user).toBe('alice@company.com');
  });

  it('updates URL when state changes', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useUrlParams(), {
      wrapper,
    });

    const [, setState] = result.current;

    act(() => {
      setState({
        search: 'new search',
        user: 'selected-user',
      });
    });

    expect(history.location.search).toContain('search=new+search');
    expect(history.location.search).toContain('user=selected-user');
  });
});
