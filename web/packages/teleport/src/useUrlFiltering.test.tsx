/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import renderHook, { act } from 'design/utils/renderHook';

import { Label, Filter } from 'teleport/types';

import useUrlFiltering, { Filterable, State } from './useUrlFiltering';

test('empty data list', () => {
  const history = createMemoryHistory({ initialEntries: ['/test'] });
  jest.spyOn(history, 'replace');

  const { current } = renderHook(() => useUrlFiltering([]), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  // Test initial values.
  expect(current.filters).toHaveLength(0);
  expect(current.result).toHaveLength(0);
});

test('extracting unique labels and making sorted filters from data list', () => {
  const label1: Label = { name: 'name100', value: 'value100' };
  const label2: Label = { name: 'name9', value: 'value9' };
  const label3: Label = { name: 'name80', value: 'value90' };
  const label4: Label = { name: 'name1', value: 'value1' };
  const label5: Label = { name: 'name10', value: 'value10' };

  const data: Filterable[] = [
    { labels: [label1] },
    { labels: [label1, label2] },
    { labels: [label3, label4] },
    { labels: [label1, label2, label3, label4] },
    { labels: [label5] },
  ];
  const history = createMemoryHistory({ initialEntries: ['/test'] });

  const { current } = renderHook(() => useUrlFiltering(data), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  expect(current.appliedFilters).toHaveLength(0);
  expect(current.result).toEqual(data);
  expect(current.filters).toHaveLength(5);

  // Test alphanum sorting.
  expect(current.filters.map(f => f.name)).toEqual([
    'name1',
    'name9',
    'name10',
    'name80',
    'name100',
  ]);

  // Test correct filter format.
  expect(current.filters[0]).toMatchObject({
    name: 'name1',
    value: 'value1',
    kind: 'label',
  });
});

test('filtering data', () => {
  const filter1: Filter = {
    name: 'name100',
    value: 'value100',
    kind: 'label',
  };
  const filter2: Filter = {
    name: 'name9',
    value: 'value9',
    kind: 'label',
  };

  const label1: Label = { name: 'name100', value: 'value100' };
  const label2: Label = { name: 'name9', value: 'value9' };
  const label3: Label = { name: 'name80', value: 'value90' };

  const data: Filterable[] = [
    { labels: [label1] },
    { labels: [label1, label2] },
    { labels: [label3] },
  ];

  const history = createMemoryHistory({ initialEntries: ['/test'] });

  const utils = renderHook(() => useUrlFiltering(data), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  let hook: State = utils.current;

  // Test initial values.
  expect(hook.appliedFilters).toHaveLength(0);
  expect(hook.filters).toHaveLength(3);
  expect(hook.result).toEqual(data);

  // Test data has been correctly filtered by applying a filter.
  act(() => hook.applyFilters([filter1]));
  hook = utils.current;
  expect(hook.result).toHaveLength(2);
  expect(hook.result[0].labels).toContainEqual(label1);
  expect(hook.result[1].labels).toContainEqual(label1);

  // Test adding another filter by toggle.
  act(() => hook.toggleFilter(filter2));
  hook = utils.current;
  expect(hook.result).toHaveLength(1);
  expect(hook.result[0].labels).toEqual(
    expect.arrayContaining([label1, label2])
  );

  // Test empty filters.
  act(() => hook.applyFilters([]));
  hook = utils.current;
  expect(hook.result).toEqual(data);
});

function Wrapper(props: any) {
  return <Router history={props.history}>{props.children}</Router>;
}
