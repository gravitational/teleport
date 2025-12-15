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

import { EventRange } from 'teleport/components/EventRangePicker';

import {
  searchParamsToState,
  statesAreEqual,
  stateToSearchParams,
  useRecordingsListState,
  type RecordingsListState,
} from './state';

describe('searchParamsToState', () => {
  const mockRanges: EventRange[] = [
    {
      from: new Date('2025-01-01'),
      to: new Date('2025-01-31'),
      name: 'Last 30 days',
    },
    {
      from: new Date('2025-01-15'),
      to: new Date('2025-01-31'),
      name: 'Last 14 days',
    },
    {
      from: new Date('2025-01-24'),
      to: new Date('2025-01-31'),
      name: 'Last 7 days',
    },
  ];

  it('returns default state when no params provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(mockRanges, params);

    expect(state).toEqual({
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    });
  });

  it('parses timeframe parameter correctly', () => {
    const params = new URLSearchParams('?timeframe=2');
    const state = searchParamsToState(mockRanges, params);

    expect(state.range).toBe(mockRanges[2]);
  });

  it('parses custom date range correctly', () => {
    const from = '2025-01-10T00:00:00.000Z';
    const to = '2025-01-20T00:00:00.000Z';

    const params = new URLSearchParams(`?from=${from}&to=${to}`);
    const state = searchParamsToState(mockRanges, params);

    expect(state.range).toEqual({
      from: new Date(from),
      to: new Date(to),
      isCustom: true,
    });
  });

  it('ignores invalid timeframe values', () => {
    const params = new URLSearchParams('?timeframe=invalid');
    const state = searchParamsToState(mockRanges, params);

    expect(state.range).toBe(mockRanges[0]);
  });

  it('ignores out of bounds timeframe values', () => {
    const params = new URLSearchParams('?timeframe=100');
    const state = searchParamsToState(mockRanges, params);

    expect(state.range).toBe(mockRanges[0]);
  });

  it('parses filters correctly', () => {
    const params = new URLSearchParams();

    params.append('resources', 'server-01');
    params.append('resources', 'server-02');
    params.append('types', 'ssh');
    params.append('types', 'desktop');
    params.append('users', 'alice');
    params.append('users', 'bob');

    const state = searchParamsToState(mockRanges, params);

    expect(state.filters).toEqual({
      resources: ['server-01', 'server-02'],
      types: ['ssh', 'desktop'],
      users: ['alice', 'bob'],
      hideNonInteractive: false,
    });
  });

  it('filters out invalid recording types', () => {
    const params = new URLSearchParams();

    params.append('types', 'ssh');
    params.append('types', 'invalid-type');
    params.append('types', 'desktop');

    const state = searchParamsToState(mockRanges, params);

    expect(state.filters.types).toEqual(['ssh', 'desktop']);
  });

  it('parses sort parameters correctly', () => {
    const params = new URLSearchParams('?sort=type&direction=ASC');
    const state = searchParamsToState(mockRanges, params);

    expect(state.sortKey).toBe('type');
    expect(state.sortDirection).toBe('ASC');
  });

  it('ignores invalid sort key', () => {
    const params = new URLSearchParams('?sort=invalid');
    const state = searchParamsToState(mockRanges, params);

    expect(state.sortKey).toBe('date');
  });

  it('ignores invalid sort direction', () => {
    const params = new URLSearchParams('?direction=INVALID');
    const state = searchParamsToState(mockRanges, params);

    expect(state.sortDirection).toBe('DESC');
  });

  it('parses page parameter correctly', () => {
    const params = new URLSearchParams('?page=5');
    const state = searchParamsToState(mockRanges, params);

    expect(state.page).toBe(5);
  });

  it('ignores invalid page values', () => {
    const params = new URLSearchParams('?page=invalid');
    const state = searchParamsToState(mockRanges, params);

    expect(state.page).toBe(0);
  });

  it('ignores negative page values', () => {
    const params = new URLSearchParams('?page=-1');
    const state = searchParamsToState(mockRanges, params);

    expect(state.page).toBe(0);
  });

  it('parses hideNonInteractive parameter when true', () => {
    const params = new URLSearchParams('?hide_non_interactive=true');
    const state = searchParamsToState(mockRanges, params);

    expect(state.filters.hideNonInteractive).toBe(true);
  });

  it('defaults hideNonInteractive to false when not provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(mockRanges, params);

    expect(state.filters.hideNonInteractive).toBe(false);
  });

  it('ignores invalid hideNonInteractive values', () => {
    const params = new URLSearchParams('?hide_non_interactive=invalid');
    const state = searchParamsToState(mockRanges, params);

    expect(state.filters.hideNonInteractive).toBe(false);
  });

  it('ignores hideNonInteractive when set to false', () => {
    const params = new URLSearchParams('?hide_non_interactive=false');
    const state = searchParamsToState(mockRanges, params);

    expect(state.filters.hideNonInteractive).toBe(false);
  });

  it('parses search parameter correctly', () => {
    const params = new URLSearchParams('?search=test%20query');
    const state = searchParamsToState(mockRanges, params);

    expect(state.search).toBe('test query');
  });

  it('defaults search to empty string when not provided', () => {
    const params = new URLSearchParams();
    const state = searchParamsToState(mockRanges, params);

    expect(state.search).toBe('');
  });

  it('handles empty search parameter', () => {
    const params = new URLSearchParams('?search=');
    const state = searchParamsToState(mockRanges, params);

    expect(state.search).toBe('');
  });
});

describe('stateToSearchParams', () => {
  const mockRanges: EventRange[] = [
    {
      from: new Date('2025-01-01'),
      to: new Date('2025-01-31'),
      name: 'Last 30 days',
    },
    {
      from: new Date('2025-01-15'),
      to: new Date('2025-01-31'),
      name: 'Last 14 days',
    },
  ];

  it('returns empty params for default state', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);

    expect(params).toBe('');
  });

  it('includes custom date range', () => {
    const from = new Date('2025-01-10T00:00:00.000Z');
    const to = new Date('2025-01-20T00:00:00.000Z');
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: { from, to, isCustom: true },
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('from')).toBe(from.toISOString());
    expect(urlParams.get('to')).toBe(to.toISOString());
  });

  it('includes all filters', () => {
    const state: RecordingsListState = {
      filters: {
        resources: ['server-01', 'server-02'],
        types: ['ssh', 'desktop'],
        users: ['alice', 'bob'],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.getAll('resources')).toEqual(['server-01', 'server-02']);
    expect(urlParams.getAll('types')).toEqual(['ssh', 'desktop']);
    expect(urlParams.getAll('users')).toEqual(['alice', 'bob']);
  });

  it('includes non-default sort parameters', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'type',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('sort')).toBe('type');
    expect(urlParams.get('direction')).toBe('ASC');
  });

  it('includes non-zero page', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 3,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    expect(params).toBe('page=3');
  });

  it('combines all parameters correctly', () => {
    const state: RecordingsListState = {
      filters: {
        resources: ['server-01'],
        types: ['ssh'],
        users: ['alice'],
        hideNonInteractive: false,
      },
      page: 2,
      range: {
        from: new Date('2025-01-10T00:00:00.000Z'),
        to: new Date('2025-01-20T00:00:00.000Z'),
        isCustom: true,
      },
      search: '',
      sortKey: 'type',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('from')).toBeTruthy();
    expect(urlParams.get('to')).toBeTruthy();
    expect(urlParams.getAll('resources')).toEqual(['server-01']);
    expect(urlParams.getAll('types')).toEqual(['ssh']);
    expect(urlParams.getAll('users')).toEqual(['alice']);
    expect(urlParams.get('sort')).toBe('type');
    expect(urlParams.get('direction')).toBe('ASC');
    expect(urlParams.get('page')).toBe('2');
  });

  it('includes hideNonInteractive when true', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: true,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('hide_non_interactive')).toBe('true');
  });

  it('omits hideNonInteractive when false', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.has('hide_non_interactive')).toBe(false);
  });

  it('includes hideNonInteractive with other filters', () => {
    const state: RecordingsListState = {
      filters: {
        resources: ['server-01'],
        types: ['ssh'],
        users: ['alice'],
        hideNonInteractive: true,
      },
      page: 1,
      range: mockRanges[0],
      search: '',
      sortKey: 'type',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('hide_non_interactive')).toBe('true');
    expect(urlParams.getAll('resources')).toEqual(['server-01']);
    expect(urlParams.getAll('types')).toEqual(['ssh']);
    expect(urlParams.getAll('users')).toEqual(['alice']);
    expect(urlParams.get('page')).toBe('1');
    expect(urlParams.get('sort')).toBe('type');
    expect(urlParams.get('direction')).toBe('ASC');
  });

  it('includes search when not empty', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: 'test query',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('search')).toBe('test query');
  });

  it('omits search when empty', () => {
    const state: RecordingsListState = {
      filters: {
        types: [],
        resources: [],
        users: [],
        hideNonInteractive: false,
      },
      page: 0,
      range: mockRanges[0],
      search: '',
      sortKey: 'date',
      sortDirection: 'DESC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.has('search')).toBe(false);
  });

  it('includes search with other parameters', () => {
    const state: RecordingsListState = {
      filters: {
        resources: ['server-01'],
        types: ['ssh'],
        users: ['alice'],
        hideNonInteractive: true,
      },
      page: 1,
      range: mockRanges[0],
      search: 'important session',
      sortKey: 'type',
      sortDirection: 'ASC',
    };

    const params = stateToSearchParams(state);
    const urlParams = new URLSearchParams(params);

    expect(urlParams.get('search')).toBe('important session');
    expect(urlParams.getAll('resources')).toEqual(['server-01']);
    expect(urlParams.getAll('types')).toEqual(['ssh']);
    expect(urlParams.getAll('users')).toEqual(['alice']);
    expect(urlParams.get('hide_non_interactive')).toBe('true');
    expect(urlParams.get('page')).toBe('1');
    expect(urlParams.get('sort')).toBe('type');
    expect(urlParams.get('direction')).toBe('ASC');
  });
});

describe('statesAreEqual', () => {
  const baseState: RecordingsListState = {
    filters: {
      types: ['ssh'],
      resources: ['server-01'],
      users: ['alice'],
      hideNonInteractive: false,
    },
    page: 1,
    range: {
      from: new Date('2025-01-01'),
      to: new Date('2025-01-31'),
      isCustom: false,
    },
    search: 'test',
    sortKey: 'date',
    sortDirection: 'DESC',
  };

  it('returns true for identical states', () => {
    const state1 = { ...baseState };
    const state2 = { ...baseState };

    expect(statesAreEqual(state1, state2)).toBe(true);
  });

  it('returns false when ranges differ', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      range: {
        from: new Date('2025-01-10'),
        to: new Date('2025-01-31'),
        isCustom: false,
      },
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns false when isCustom differs', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      range: { ...baseState.range, isCustom: true },
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns false when filters differ', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      filters: {
        ...baseState.filters,
        users: ['alice', 'bob'],
      },
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns false when sort parameters differ', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      sortKey: 'type' as const,
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns false when page differs', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      page: 2,
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns true when filter arrays have same values in same order', () => {
    const state1 = { ...baseState };
    const state2 = { ...baseState };

    expect(statesAreEqual(state1, state2)).toBe(true);
  });

  it('returns false when hideNonInteractive differs', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      filters: {
        ...baseState.filters,
        hideNonInteractive: true,
      },
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns true when hideNonInteractive is the same', () => {
    const state1 = {
      ...baseState,
      filters: {
        ...baseState.filters,
        hideNonInteractive: true,
      },
    };
    const state2 = {
      ...baseState,
      filters: {
        ...baseState.filters,
        hideNonInteractive: true,
      },
    };

    expect(statesAreEqual(state1, state2)).toBe(true);
  });

  it('returns false when search differs', () => {
    const state1 = { ...baseState };
    const state2 = {
      ...baseState,
      search: 'different search',
    };

    expect(statesAreEqual(state1, state2)).toBe(false);
  });

  it('returns true when search is the same', () => {
    const state1 = {
      ...baseState,
      search: 'same query',
    };
    const state2 = {
      ...baseState,
      search: 'same query',
    };

    expect(statesAreEqual(state1, state2)).toBe(true);
  });
});

describe('useRecordingsListState', () => {
  const mockRanges: EventRange[] = [
    {
      from: new Date('2025-01-01'),
      to: new Date('2025-01-31'),
      name: 'Last 30 days',
    },
  ];

  it('initializes state from URL search params', () => {
    const history = createMemoryHistory({
      initialEntries: ['/session-recordings?page=2&sort=type'],
    });

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [state] = result.current;

    expect(state.page).toBe(2);
    expect(state.sortKey).toBe('type');
  });

  it('updates URL when state changes', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [, setState] = result.current;

    act(() => {
      setState(prev => ({
        ...prev,
        page: 3,
        sortKey: 'type',
      }));
    });

    expect(history.location.search).toContain('page=3');
    expect(history.location.search).toContain('sort=type');
  });

  it('responds to browser navigation', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    act(() => {
      history.push('?page=5&sort=type');
    });

    const [state] = result.current;

    expect(state.page).toBe(5);
    expect(state.sortKey).toBe('type');
  });

  it('prevents unnecessary state updates', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [initialState, setState] = result.current;

    act(() => {
      setState(initialState);
    });

    const [newState] = result.current;

    expect(newState).toBe(initialState);
  });

  it('handles functional updates', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [, setState] = result.current;

    act(() => {
      setState(prev => ({
        ...prev,
        page: prev.page + 1,
      }));
    });

    const [state] = result.current;

    expect(state.page).toBe(1);
  });

  it('does not update URL for default empty search params', () => {
    const history = createMemoryHistory();
    const replaceSpy = jest.spyOn(history, 'replace');

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    renderHook(() => useRecordingsListState(mockRanges), { wrapper });

    expect(replaceSpy).not.toHaveBeenCalled();
  });

  it('handles hideNonInteractive filter in URL params', () => {
    const history = createMemoryHistory({
      initialEntries: ['/session-recordings?hide_non_interactive=true'],
    });

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [state] = result.current;

    expect(state.filters.hideNonInteractive).toBe(true);
  });

  it('updates URL when hideNonInteractive filter changes', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [, setState] = result.current;

    act(() => {
      setState(prev => ({
        ...prev,
        filters: {
          ...prev.filters,
          hideNonInteractive: true,
        },
      }));
    });

    expect(history.location.search).toContain('hide_non_interactive=true');
  });

  it('handles search parameter in URL params', () => {
    const history = createMemoryHistory({
      initialEntries: ['/session-recordings?search=test%20query'],
    });

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [state] = result.current;

    expect(state.search).toBe('test query');
  });

  it('updates URL when search changes', () => {
    const history = createMemoryHistory();

    function wrapper({ children }: PropsWithChildren) {
      return <Router history={history}>{children}</Router>;
    }

    const { result } = renderHook(() => useRecordingsListState(mockRanges), {
      wrapper,
    });

    const [, setState] = result.current;

    act(() => {
      setState(prev => ({
        ...prev,
        search: 'new search term',
      }));
    });

    expect(history.location.search).toContain('search=new+search+term');
  });
});
