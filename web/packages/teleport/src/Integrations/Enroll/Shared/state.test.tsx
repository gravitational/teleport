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
import type { PropsWithChildren } from 'react';
import { MemoryRouter, useLocation, useNavigate } from 'react-router';

import {
  searchParamsToState,
  stateToSearchParams,
  useIntegrationPickerState,
  type IntegrationPickerState,
} from './state';

describe('searchParamsToState', () => {
  it('returns default state when no params provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(params);

    expect(state).toEqual({
      filters: {
        tags: [],
      },
      search: '',
      sortKey: undefined,
      sortDirection: 'ASC',
    });
  });

  it('parses filters correctly', () => {
    const params = new URLSearchParams();

    params.append('tags', 'bot');
    params.append('tags', 'cicd');

    const state = searchParamsToState(params);

    expect(state.filters).toEqual({
      tags: ['bot', 'cicd'],
    });
  });

  it('filters out invalid integration tags', () => {
    const params = new URLSearchParams();

    params.append('tags', 'bot');
    params.append('tags', 'invalid-type');
    params.append('tags', 'cicd');

    const state = searchParamsToState(params);

    expect(state.filters.tags).toEqual(['bot', 'cicd']);
  });

  it('parses sort parameters correctly', () => {
    const params = new URLSearchParams('?sort=name&direction=ASC');
    const state = searchParamsToState(params);

    expect(state.sortKey).toBe('name');
    expect(state.sortDirection).toBe('ASC');
  });

  it('ignores invalid sort key', () => {
    const params = new URLSearchParams('?sort=invalid');
    const state = searchParamsToState(params);

    expect(state.sortKey).toBe(undefined);
  });

  it('ignores invalid sort direction', () => {
    const params = new URLSearchParams('?direction=INVALID');
    const state = searchParamsToState(params);

    expect(state.sortDirection).toBe('ASC');
  });

  it('parses search parameter correctly', () => {
    const params = new URLSearchParams('?search=test%20query');
    const state = searchParamsToState(params);

    expect(state.search).toBe('test query');
  });

  it('defaults search to empty string when not provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(params);

    expect(state.search).toBe('');
  });

  it('handles empty search parameter', () => {
    const params = new URLSearchParams('?search=');
    const state = searchParamsToState(params);

    expect(state.search).toBe('');
  });
});

describe('stateToSearchParams', () => {
  it('returns empty params for default state', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: [],
      },
      search: '',
      sortKey: undefined,
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);

    expect(params).toBe('');
  });

  it('includes all filters', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: ['bot', 'cicd'],
      },
      search: '',
      sortKey: 'name',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.getAll('tags')).toEqual(['bot', 'cicd']);
  });

  it('combines all parameters correctly', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: ['bot'],
      },
      search: '',
      sortKey: 'name',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.getAll('tags')).toEqual(['bot']);
    expect(urlParams.get('sort')).toBe('name');
    expect(urlParams.get('direction')).toBe('ASC');
  });

  it('includes search when not empty', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: [],
      },
      search: 'test query',
      sortKey: 'name',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('search')).toBe('test query');
  });

  it('omits search when empty', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: [],
      },
      search: '',
      sortKey: 'name',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.has('search')).toBe(false);
  });

  it('includes search with other parameters', () => {
    const state: IntegrationPickerState = {
      filters: {
        tags: ['bot'],
      },
      search: 'google cloud',
      sortKey: 'name',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('search')).toBe('google cloud');
    expect(urlParams.getAll('tags')).toEqual(['bot']);
    expect(urlParams.get('sort')).toBe('name');
    expect(urlParams.get('direction')).toBe('ASC');
  });
});

describe('useIntegrationPickerState', () => {
  function createWrapper(initialEntries: string[] = ['/']) {
    return function wrapper({ children }: PropsWithChildren) {
      return (
        <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
      );
    };
  }

  function useIntegrationPickerStateHarness() {
    const [state, setState] = useIntegrationPickerState();
    const location = useLocation();
    const navigate = useNavigate();

    return {
      state,
      setState,
      location,
      navigate,
    };
  }

  it('initializes state from URL search params', () => {
    const wrapper = createWrapper(['/integrations/new?search=cool']);

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    expect(result.current.state.search).toBe('cool');
  });

  it('updates URL when state changes', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    act(() => {
      result.current.setState(prev => ({
        ...prev,
        sortKey: 'name',
        sortDirection: 'DESC',
      }));
    });

    expect(result.current.location.search).toContain('direction=DESC');
    expect(result.current.location.search).toContain('sort=name');
  });

  it('responds to browser navigation', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    act(() => {
      result.current.navigate('?sort=name&direction=DESC');
    });

    expect(result.current.state.sortKey).toBe('name');
    expect(result.current.state.sortDirection).toBe('DESC');
  });

  it('prevents unnecessary state updates', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    const { state: initialState, setState } = result.current;

    act(() => {
      setState(initialState);
    });

    expect(result.current.state).toBe(initialState);
  });

  it('handles functional updates', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    act(() => {
      result.current.setState(prev => ({
        ...prev,
        filters: {
          ...prev.filters,
          tags: ['cicd'],
        },
      }));
    });

    expect(result.current.state.filters.tags.length).toBe(1);
  });

  it('does not update URL for default empty search params', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    // With empty state and no initial search params, URL should remain empty
    expect(result.current.location.search).toBe('');
  });

  it('handles search parameter in URL params', () => {
    const wrapper = createWrapper(['/integrations/new?search=test%20query']);

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    expect(result.current.state.search).toBe('test query');
  });

  it('updates URL when search changes', () => {
    const wrapper = createWrapper();

    const { result } = renderHook(() => useIntegrationPickerStateHarness(), {
      wrapper,
    });

    act(() => {
      result.current.setState(prev => ({
        ...prev,
        search: 'new search term',
      }));
    });

    expect(result.current.location.search).toContain('search=new+search+term');
  });
});
