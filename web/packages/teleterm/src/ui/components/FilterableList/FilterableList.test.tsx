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
import { screen, within } from '@testing-library/react';
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

test('render all items by default', () => {
  render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );
  const items = screen.getAllByRole('listitem');

  expect(items).toHaveLength(30);
  items.forEach((item, index) => {
    expect(item).toHaveTextContent(mockedItems[index].title);
  });
});

test('render a item that matches the search', () => {
  render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );
  fireEvent.change(screen.getByRole('searchbox'), {
    target: { value: mockedItems[0].title },
  });

  const item = screen.getByRole('listitem');

  expect(within(item).getByText(mockedItems[0].title)).toBeInTheDocument();
});

test('render empty list when search does not match any item', () => {
  render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );

  fireEvent.change(screen.getByRole('searchbox'), {
    target: { value: 'abc' },
  });

  expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
});

test('render provided placeholder in the search box', () => {
  const placeholder = 'Search Connections';
  render(
    <FilterableList<TestItem>
      items={[]}
      filterBy="title"
      placeholder={placeholder}
      Node={Node}
    />
  );

  expect(screen.getByPlaceholderText(placeholder)).toBeInTheDocument();
});
