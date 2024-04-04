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

import React, { Fragment, ReactNode, useMemo, useState } from 'react';
import { Input } from 'design';
import styled from 'styled-components';

interface FilterableListProps<T> {
  items: T[];
  filterBy: keyof T;
  placeholder?: string;

  Node(props: { item: T; index: number }): ReactNode;

  onFilterChange?(filter: string): void;
}

export function FilterableList<T>(props: FilterableListProps<T>) {
  const { items } = props;
  const [searchValue, setSearchValue] = useState<string>();

  const filteredItems = useMemo(
    () => filterItems(searchValue, items, props.filterBy),
    [items, searchValue]
  );

  return (
    <>
      <StyledInput
        role="searchbox"
        onChange={e => {
          const { value } = e.target;
          props.onFilterChange?.(value);
          setSearchValue(value);
        }}
        placeholder={props.placeholder}
        autoFocus={true}
      />
      <UnorderedList>
        {filteredItems.map((item, index) => (
          <Fragment key={index}>{props.Node({ item, index })}</Fragment>
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
  const trimmed = searchValue?.trim().toLocaleLowerCase();
  if (!trimmed) {
    return items;
  }
  return items.filter(item =>
    item[filterBy].toString().toLocaleLowerCase().includes(trimmed)
  );
}

const UnorderedList = styled.ul`
  padding: 0;
  margin: 0;
`;

const StyledInput = styled(Input)`
  background-color: inherit;
  border-radius: 51px;
  margin-bottom: 8px;
  font-size: 14px;
  height: 34px;
`;
