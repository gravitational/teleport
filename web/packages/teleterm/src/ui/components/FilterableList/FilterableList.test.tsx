/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { fireEvent, render } from 'design/utils/testing';

import { FilterableList } from './FilterableList';

interface TestItem {
  title: string;
}

const mockedItems: TestItem[] = Array.from(new Array(30))
  .fill(0)
  .map((_, index) => ({ title: `Test item: ${index}` }));

function Node({ item }: { item: TestItem }) {
  return <li>{item.title}</li>;
}

test('render first 10 items by default', () => {
  const { getAllByRole } = render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );
  const items = getAllByRole('listitem');

  expect(items).toHaveLength(10);
  items.forEach((item, index) => {
    expect(item).toHaveTextContent(mockedItems[index].title);
  });
});

test('render a item that matches the search', () => {
  const { getAllByRole, getByRole } = render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );
  fireEvent.change(getByRole('searchbox'), {
    target: { value: mockedItems[0].title },
  });
  const items = getAllByRole('listitem');

  expect(items).toHaveLength(1);
  expect(items[0]).toHaveTextContent(mockedItems[0].title);
});

test('render empty list when search does not match any item', () => {
  const { queryAllByRole, getByRole } = render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );

  fireEvent.change(getByRole('searchbox'), {
    target: { value: 'abc' },
  });
  const items = queryAllByRole('listitem');

  expect(items).toHaveLength(0);
});

test('render provided placeholder in the search box', () => {
  const placeholder = 'Search Connections';
  const { getByPlaceholderText } = render(
    <FilterableList<TestItem>
      items={[]}
      filterBy="title"
      placeholder={placeholder}
      Node={Node}
    />
  );

  expect(getByPlaceholderText(placeholder)).toBeInTheDocument();
});
