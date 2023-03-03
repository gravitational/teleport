/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import renderHook from 'design/utils/renderHook';
import { SortType } from 'design/DataTable/types';

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
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialSort), {
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
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialSort), {
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
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialSort), {
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
  };

  const history = createMemoryHistory({ initialEntries: [url] });

  let result;
  result = renderHook(() => useUrlFiltering(initialSort), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(result.current.params).toEqual(expected);
});

const initialSort: SortType = {
  fieldName: 'description',
  dir: 'ASC',
};

function Wrapper(props: any) {
  return <Router history={props.history}>{props.children}</Router>;
}
