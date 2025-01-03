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

import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { SortType } from 'design/DataTable/types';
import renderHook from 'design/utils/renderHook';

import { useUrlFiltering } from './useUrlFiltering';

test('extracting params from URL with simple search and sort params', () => {
  const url = '/test?search=test%20multiple%20words&sort=name:desc';
  const expected = {
    search: 'test multiple words',
    sort: {
      fieldName: 'name',
      dir: 'DESC',
    },
    query: null,
    kinds: null,
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialParams), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(result.current.params).toEqual(expected);
});

test('extracting params from URL with advanced search and sort params', () => {
  const url =
    '/test?query=labels.env%20%3D%3D%20"test"%20%26%26%20labels%5B"test-cluster"%5D%20%3D%3D%20"one"%20%7C%7C%20search("apple")&sort=name:desc';
  const expected = {
    query:
      'labels.env == "test" && labels["test-cluster"] == "one" || search("apple")',
    sort: {
      fieldName: 'name',
      dir: 'DESC',
    },
    search: null,
    kinds: null,
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialParams), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(result.current.params).toEqual(expected);
});

test('extracting params from URL with simple search param but no sort param', () => {
  const url =
    '/test?search=test!%20special%20characters%20are%20"totally"%20100%25%20cool%20%24_%24';
  const expected = {
    search: 'test! special characters are "totally" 100% cool $_$',
    sort: initialSort,
    query: null,
    kinds: null,
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialParams), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(result.current.params).toEqual(expected);
});

test('extracting params from URL with no search param and with sort param with undefined sortdir', () => {
  const url = '/test?sort=name';
  const expected = {
    sort: { fieldName: 'name', dir: 'ASC' },
    search: null,
    query: null,
    kinds: null,
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialParams), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(result.current.params).toEqual(expected);
});

test('extracting params from URL with resource kinds', () => {
  const url = '/test?kinds=node&kinds=db';
  const expected = {
    kinds: ['node', 'db'],
    search: null,
    sort: initialSort,
    query: null,
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  const { current } = renderHook(() => useUrlFiltering(initialParams), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(current.params).toEqual(expected);
});

const initialSort: SortType = {
  fieldName: 'description',
  dir: 'ASC',
};

const initialParams = { sort: initialSort };

function Wrapper(props: any) {
  return <Router history={props.history}>{props.children}</Router>;
}
