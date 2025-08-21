/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { SortType } from 'design/DataTable/types';
import * as Icons from 'design/Icon';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { SortMenu } from 'shared/components/Controls/SortMenu';

import { type IntegrationTag } from 'teleport/Integrations/Enroll/IntegrationTiles/integrations';

export function FilterPanel({ integrationTagOptions, params, setParams }) {
  const { sort, kinds } = params;

  const sortFieldOptions = [{ label: 'Name', value: 'name' }];

  const defaultSort: SortType = { dir: 'ASC', fieldName: 'name' };

  const activeSort = sort || defaultSort;

  const activeSortFieldOption = sortFieldOptions.find(
    opt => opt.value === activeSort.fieldName
  );

  function oppositeSort(sort: SortType): SortType {
    switch (sort?.dir) {
      case 'ASC':
        return { ...sort, dir: 'DESC' };
      case 'DESC':
        return { ...sort, dir: 'ASC' };
      default:
        return sort;
    }
  }

  const onSortFieldChange = (value: string) => {
    setParams({ ...params, sort: { ...activeSort, fieldName: value } });
  };

  const onSortOrderButtonClicked = () => {
    setParams({ ...params, sort: oppositeSort(activeSort) });
  };

  function setSearch(search: string) {
    setParams({ ...params, search: search });
  }

  return (
    <>
      <Box maxWidth="600px" width="100%">
        <DebouncedSearchInput
          onSearch={value => {
            setSearch(value);
          }}
          placeholder={'Search for integrations...'}
          initialValue={params.search || ''}
        />
      </Box>
      <Flex justifyContent="space-between" minWidth="419px">
        <Flex justifyContent="flex-start">
          <MultiselectMenu
            options={integrationTagOptions}
            onChange={integrationTags =>
              setParams({ ...params, kinds: integrationTags as string[] })
            }
            selected={(kinds as IntegrationTag[]) || []}
            label="Integration Type"
            tooltip="Filter by integration type"
          />
        </Flex>
        <Flex justifyContent="flex-end">
          <SortMenu
            current={{
              fieldName: activeSortFieldOption.value,
              dir: activeSort.dir,
            }}
            fields={sortFieldOptions}
            onChange={newSort => {
              if (newSort.dir !== activeSort.dir) {
                onSortOrderButtonClicked();
              }
              if (newSort.fieldName !== activeSort.fieldName) {
                onSortFieldChange(newSort.fieldName);
              }
            }}
          />
        </Flex>
      </Flex>
    </>
  );
}

const DebouncedSearchInput = ({
  onSearch,
  placeholder = '',
  initialValue = '',
}: {
  onSearch: (searchValue: string) => void;
  placeholder?: string;
  initialValue?: string;
}) => {
  const [searchTerm, setSearchTerm] = useState(initialValue);
  const [debouncedTerm, setDebouncedTerm] = useState('');
  const isFirstRender = useRef(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedTerm(searchTerm);
    }, 350);

    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => {
    if (isFirstRender.current && debouncedTerm === '') {
      isFirstRender.current = false;
      return;
    }

    onSearch(debouncedTerm);
  }, [debouncedTerm]);

  return (
    <InputWrapper
      onSubmit={e => {
        e.preventDefault();
        onSearch(searchTerm);
      }}
    >
      <Icons.Magnifier size={16} color="text.slightlyMuted" />
      <StyledInput
        placeholder={placeholder}
        autoFocus
        max={100}
        name="searchValue"
        value={searchTerm}
        onChange={e => setSearchTerm(e.target.value)}
      />
    </InputWrapper>
  );
};

const InputWrapper = styled.form`
  border-radius: ${props => props.theme.radii[5]}px;
  height: 40px;
  border: 1px solid ${props => props.theme.colors.interactive.tonal.neutral[2]};
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  padding: 0 ${props => props.theme.space[3]}px 0
    ${props => props.theme.space[3]}px;
  background-color: transparent;
  transition:
    background-color 150ms ease,
    border-color 150ms ease;

  &:focus-within,
  &:active {
    border-color: ${p => p.theme.colors.brand};
  }

  &:hover,
  &:focus-within,
  &:active {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  height: 100%;
  width: 100%;
  transition: all 200ms ease;
  color: ${props => props.theme.colors.text.main};
  background: transparent;
  padding: ${props => props.theme.space[3]}px ${props => props.theme.space[3]}px
    ${props => props.theme.space[3]}px ${props => props.theme.space[2]}px;
  flex: 1;
`;
