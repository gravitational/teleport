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

import { Filter } from 'teleport/types';

import useUrlQueryParams, { State } from './useUrlQueryParams';

test('apply filters and correct encoding of url', () => {
  const history = createMemoryHistory({ initialEntries: ['/test'] });
  jest.spyOn(history, 'replace');

  const utils = renderHook(() => useUrlQueryParams(), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  let hook: State = utils.current;

  // Test initial values.
  expect(hook.filters).toHaveLength(0);

  // Test add a filter.
  act(() => hook.applyFilters([filter1]));
  hook = utils.current;
  expect(hook.filters).toHaveLength(1);
  expect(hook.filters).toMatchObject([filter1]);

  // Test applied filter has been correctly encoded into url.
  let expectedURL = `/test?q=l=${encodedLabel1.name}:${encodedLabel1.value}`;
  expect(history.replace).toHaveBeenCalledWith(expectedURL);
  jest.clearAllMocks();

  // Test applying multiple filters.
  act(() => hook.applyFilters([filter1, filter2]));
  hook = utils.current;
  expect(hook.filters).toHaveLength(2);
  expect(hook.filters).toEqual(expect.arrayContaining([filter1, filter2]));

  // Test url has been encoded correctly.
  expectedURL = `/test?q=l=${encodedLabel1.name}:${encodedLabel1.value}+l=${encodedLabel2.name}:${encodedLabel2.value}`;
  expect(history.replace).toHaveBeenCalledWith(expectedURL);

  // Test empty array.
  act(() => hook.applyFilters([]));
  hook = utils.current;
  expect(hook.filters).toHaveLength(0);
  expect(history.replace).toHaveBeenCalledWith(`/test`);
});

test('toggle filter', () => {
  const baseUrl = `/test?q=l=${encodedLabel1.name}:${encodedLabel1.value}+l=${encodedLabel2.name}:${encodedLabel2.value}`;
  const history = createMemoryHistory({ initialEntries: [baseUrl] });
  jest.spyOn(history, 'replace');

  const utils = renderHook(() => useUrlQueryParams(), {
    wrapper: Wrapper,
    wrapperProps: { history },
  });

  let hook: State = utils.current;

  // Test initial values.
  expect(hook.filters).toHaveLength(2);
  expect(hook.filters).toEqual(expect.arrayContaining([filter1, filter2]));

  // Test toggling existing label (delete).
  act(() => hook.toggleFilter(filter2));
  hook = utils.current;
  expect(hook.filters).toHaveLength(1);
  expect(hook.filters).toEqual(expect.arrayContaining([filter1]));

  // Test url has been updated correctly.
  let expectedURL = `/test?q=l=${encodedLabel1.name}:${encodedLabel1.value}`;
  expect(history.replace).toHaveBeenCalledWith(expectedURL);

  // Test toggling new label (add).
  act(() => hook.toggleFilter(filter2));
  hook = utils.current;
  expect(hook.filters).toHaveLength(2);
  expect(hook.filters).toEqual(expect.arrayContaining([filter1, filter2]));

  // Test url has been updated correctly.
  expect(history.replace).toHaveBeenCalledWith(baseUrl);
});

describe('test decoding of urls', () => {
  const enc = encodeURIComponent;

  const testCases: {
    name: string;
    url: string;
    expect: Filter[];
  }[] = [
    {
      name: 'no filter param',
      url: '/test?',
      expect: [],
    },
    {
      name: 'no filters',
      url: '/test?q=',
      expect: [],
    },
    {
      name: 'blank label',
      url: '/test?q=l=',
      expect: [],
    },
    {
      name: 'blank label with colon delimiter',
      url: '/test?q=l=:',
      expect: [],
    },
    {
      name: 'blank label with plus delimiter',
      url: '/test?q=l=+',
      expect: [],
    },
    {
      name: 'blank label with all delimiters',
      url: '/test?q=l=:+',
      expect: [],
    },
    {
      name: 'unknown filter type',
      url: `/test?q=unknown=${enc('k')}:${enc('v')}`,
      expect: [],
    },
    {
      name: 'malformed label value (double delimiter)',
      url: `/test?q=l=${enc('k')}:${enc('v')}:extra`,
      expect: [],
    },
    {
      name: 'skip unknown filter identifier',
      url: `/test?q=l=${enc('k')}:${enc('v')}+unkwn=${enc('k2')}:${enc('v2')}`,
      expect: [{ name: 'k', value: 'v', kind: 'label' }],
    },
    {
      name: 'missing label delimiter',
      url: `/test?q=l=${enc('k')}${enc('v')}`,
      expect: [{ name: 'kv', value: '', kind: 'label' }],
    },
    {
      name: 'pre label delimiter',
      url: `/test?q=l=:${enc('k')}${enc('v')}`,
      expect: [{ name: '', value: 'kv', kind: 'label' }],
    },
    {
      name: 'delimiters in encoded label does not affect unencoded delimiters',
      url: `/test?q=l=${enc('l=b:c')}:${enc(':d+e+f')}`,
      expect: [{ name: 'l=b:c', value: ':d+e+f', kind: 'label' }],
    },
    {
      name: 'valid query',
      url: `
        /test?q=l=${enc('k')}:${enc('v')}+l=${enc('k2')}:${enc('v2')}`,
      expect: [
        { name: 'k', value: 'v', kind: 'label' },
        { name: 'k2', value: 'v2', kind: 'label' },
      ],
    },
    {
      name: 'ignore blank label identifiers',
      url: `
        /test?q=l=${enc('k')}:${enc('v')}+l=${enc('k2')}:${enc('v2')}+l=+l=`,
      expect: [
        { name: 'k', value: 'v', kind: 'label' },
        { name: 'k2', value: 'v2', kind: 'label' },
      ],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const { current } = renderHook(() => useUrlQueryParams(), {
      wrapper: Wrapper,
      wrapperProps: {
        history: createMemoryHistory({ initialEntries: [testCase.url] }),
      },
    });

    expect(current.filters).toMatchObject(testCase.expect);
  });
});

function Wrapper(props: any) {
  return <Router history={props.history}>{props.children}</Router>;
}

const filter1: Filter = {
  name: 'env',
  value: 'staging',
  kind: 'label',
};
const filter2: Filter = {
  name: 'country',
  value: 'South Korea',
  kind: 'label',
};

const encodedLabel1 = {
  name: encodeURIComponent(filter1.name),
  value: encodeURIComponent(filter1.value),
};

const encodedLabel2 = {
  name: encodeURIComponent(filter2.name),
  value: encodeURIComponent(filter2.value),
};
