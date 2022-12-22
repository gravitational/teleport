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

test('render first 10 items by default', () => {
  render(
    <FilterableList<TestItem>
      items={mockedItems}
      filterBy="title"
      Node={Node}
    />
  );
  const items = screen.getAllByRole('listitem');

  expect(items).toHaveLength(10);
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
