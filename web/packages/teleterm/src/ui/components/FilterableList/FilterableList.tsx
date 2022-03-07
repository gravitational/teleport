import React, { Fragment, ReactNode, useMemo, useState } from 'react';
import { Input } from 'design';
import styled from 'styled-components';

interface FilterableListProps<T> {
  items: T[];
  filterBy: keyof T;
  placeholder?: string;

  Node(props: { item: T }): ReactNode;
}

const maxItemsToShow = 10;

export function FilterableList<T>(props: FilterableListProps<T>) {
  const { items } = props;
  const [searchValue, setSearchValue] = useState<string>();

  const filteredItems = useMemo(
    () =>
      filterItems(searchValue, items, props.filterBy).slice(0, maxItemsToShow),
    [items, searchValue]
  );

  return (
    <>
      <DarkInput
        role="searchbox"
        onChange={e => setSearchValue(e.target.value)}
        placeholder={props.placeholder}
        autoFocus={true}
      />
      <UnorderedList>
        {filteredItems.map((item, index) => (
          <Fragment key={index}>{props.Node({ item })}</Fragment>
        ))}
      </UnorderedList>
    </>
  );
}

function filterItems<T>(
  searchValue: string,
  items: T[],
  filterBy: keyof T
): T[] {
  const trimmed = searchValue?.trim();
  if (!trimmed) {
    return items;
  }
  return items.filter(item => item[filterBy].toString().includes(trimmed));
}

const UnorderedList = styled.ul`
  padding: 0;
  margin: 0;
`;

const DarkInput = styled(Input)`
  background: inherit;
  border: 1px ${props => props.theme.colors.light} solid;
  color: ${props => props.theme.colors.light};
  margin-bottom: 10px;
  font-size: 14px;
  opacity: 0.6;

  ::placeholder {
    opacity: 1;
  }
`;
