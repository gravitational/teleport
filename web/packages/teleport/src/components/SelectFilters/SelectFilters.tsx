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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { components } from 'react-select';
import styled from 'styled-components';

import { Box, ButtonBorder, ButtonIcon, Flex, Text } from 'design';
import { Add, Cross } from 'design/Icon';
import Select, {
  ActionMeta,
  Option as BaseOption,
} from 'shared/components/Select';

import { makeLabelTag } from 'teleport/components/formatters';
import { Filter } from 'teleport/types';

import Pager from './Pager';
import usePages from './usePages';

export default function SelectFilters({
  applyFilters,
  appliedFilters = [],
  filters = [],
  mb = 3,
  pageSize = 100,
}: Props) {
  const selectWrapperRef = useRef(null);
  const [showSelector, setShowSelector] = useState(false);
  const [search, setSearch] = useState('');
  const [selectedOptions, setSelectedOptions] = useState<Option[]>(() =>
    makeOptions(appliedFilters)
  );

  const options = useMemo(() => makeOptions(filters), [filters]);
  const filteredOptions = useMemo(() => {
    if (!search) {
      return options;
    }

    const searchValue = search.toLocaleLowerCase();
    return options.filter(opt => {
      const targetValue = opt.value.toLocaleLowerCase();
      return targetValue.indexOf(searchValue) !== -1;
    });
  }, [search]);

  const pagedState = usePages({ data: filteredOptions, pageSize });

  function clearOptions() {
    setSelectedOptions([]);
  }

  function deleteFilter(filter: Filter) {
    const updatedFilters = selectedOptions
      .filter(o => {
        const currFilter = `${o.filter.name}${o.filter.value}`;
        const targetFilter = `${filter.name}${filter.value}`;
        return currFilter !== targetFilter;
      })
      .map(o => o.filter);

    applyFilters(updatedFilters);
  }

  function onFilterApply() {
    setShowSelector(false);
    applyFilters(selectedOptions.map(o => o.filter));
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') {
      setShowSelector(false);
    }
  }

  function onInputChange(value: string, meta: ActionMeta) {
    if (meta.action === 'menu-close' || meta.action === 'set-value') {
      return;
    }
    setSearch(value);
  }

  useEffect(() => {
    const appliedOptions = makeOptions(appliedFilters);
    setSelectedOptions(appliedOptions);

    function handleOnClick(e) {
      // Ignore event for clicking near buttons.
      if (e.target.closest('button')) {
        return;
      }

      // If event is not from inside the select wrapper, close the selector.
      // Clicking outside is considered "canceled", so we also reset the
      // selected options back to original.
      if (!selectWrapperRef.current?.contains(e.target)) {
        setShowSelector(false);
        setSelectedOptions(appliedOptions);
      }
    }

    window.addEventListener('click', handleOnClick);
    return () => window.removeEventListener('click', handleOnClick);
  }, [appliedFilters]);

  const $labels = appliedFilters.map((f, key) => {
    let labelTxt = f.value;
    switch (f.kind) {
      case 'label':
        labelTxt = makeLabelTag({ name: f.name, value: f.value });
    }

    return <Label key={key} name={labelTxt} onClick={() => deleteFilter(f)} />;
  });

  return (
    <Flex flexWrap="wrap" mb={mb} style={{ flexShrink: '0' }}>
      <Box style={{ position: 'relative' }}>
        <AddButton
          pl={2}
          pr={3}
          onClick={() => setShowSelector(!showSelector)}
          mt={0}
          mr={3}
          mb={2}
        >
          <Add size="small" mr={1} />
          Add Filters
        </AddButton>
        {showSelector && (
          <Box
            mt={-2}
            bg="#fff"
            borderRadius={2}
            borderTopLeftRadius={0}
            style={{ position: 'absolute', zIndex: 1, color: '#4b4b4b' }}
            ref={selectWrapperRef}
          >
            <StyledSelect>
              <Select
                autoFocus
                placeholder="Search..."
                inputValue={search}
                value={selectedOptions}
                options={pagedState.data}
                isSearchable={true}
                isClearable={false}
                isMulti={true}
                menuIsOpen={true}
                hideSelectedOptions={false}
                controlShouldRenderValue={false}
                filterOption={() => true}
                onChange={(o: Option[]) => o && setSelectedOptions(o)}
                onKeyDown={handleKeyDown}
                onInputChange={onInputChange}
                components={{
                  Option: OptionComponent,
                  Control: ControlComponent,
                }}
                customProps={{
                  onFilterApply,
                  appliedFilters,
                  selectedOptions,
                  clearOptions,
                }}
              />
            </StyledSelect>
            <Pager {...pagedState} />
          </Box>
        )}
      </Box>
      {$labels}
    </Flex>
  );
}

function makeOptions(filters: Filter[] = []): Option[] {
  return filters.map(filter => {
    switch (filter.kind) {
      case 'label':
        const tag = makeLabelTag({ name: filter.name, value: filter.value });
        return { label: tag, value: tag, filter };
    }
  });
}

const ControlComponent = props => {
  const { onFilterApply, appliedFilters, selectedOptions, clearOptions } =
    props.selectProps.customProps;

  const numFilters =
    selectedOptions.length > 0 ? ` (${selectedOptions.length})` : '';

  return (
    <Flex alignItems="center">
      <components.Control {...props} />
      <Box>
        <ActionButton
          px={2}
          mr={2}
          onClick={onFilterApply}
          disabled={appliedFilters.length === 0 && selectedOptions.length === 0}
          width="90px"
        >
          Apply{numFilters}
        </ActionButton>
        <ActionButton
          px={2}
          onClick={clearOptions}
          disabled={selectedOptions.length === 0}
        >
          Clear
        </ActionButton>
      </Box>
    </Flex>
  );
};

const OptionComponent = props => {
  return (
    <components.Option {...props} className="react-select__selected">
      <Flex alignItems="center">
        <input type="checkbox" checked={props.isSelected} readOnly />{' '}
        <Text ml={1}>{props.label}</Text>
      </Flex>
    </components.Option>
  );
};

function Label({
  name,
  onClick,
}: {
  name: string;
  onClick(name: string): void;
}) {
  return (
    <StyledLabel onClick={() => onClick(name)}>
      <span title={name}>{name}</span>
      <ButtonIcon size={0} ml="1" bg="levels.surface">
        <Cross size="small" />
      </ButtonIcon>
    </StyledLabel>
  );
}

const ActionButton = styled(ButtonBorder)`
  line-height: normal;
  color: #4b4b4b;
  background-color: #fff;
  border: 1px solid #4b4b4b;
  &:hover,
  &:focus {
    border: 1px solid #4b4b4b;
    color: #000;
    background-color: #fff;
  }

  &:focus {
    border-width: 2px;
  }

  &:disabled {
    border: 1px solid #bbbbbb;
    color: #bbbbbb;
  }
`;

const AddButton = styled(ButtonBorder)`
  line-height: normal;
  background-color: ${props => props.theme.colors.levels.sunken};
  font-weight: normal;
  border: 1px solid rgba(255, 255, 255, 0.24);
  color: #fff;

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.levels.elevated};
  }
`;

const StyledSelect = styled.div`
  width: 700px;

  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__indicators {
    display: none;
  }

  .react-select__control {
    border-color: #cccccc;
    margin: 14px;
    width: 500px;
    height: 33px;
    min-height: 33px;

    &:hover {
      cursor: text;
      border-color: #cccccc;
    }
  }

  .react-select__menu {
    position: relative;
    border-top-left-radius: 0;
    border-top-right-radius: 0;
    margin-bottom: 0;
    box-shadow: none;
    border-top: 1px #dddddd solid;
    border-bottom: 1px #dddddd solid;
  }

  .react-select-container {
    box-shadow: none;
  }

  .react-select__value-container {
    padding: 0 8px;
  }
`;

const StyledLabel = styled.div`
  display: flex;
  align-items: center;
  cursor: pointer;
  line-height: 16px;
  height: 30px;
  max-width: 200px;
  margin-right: 16px;
  margin-bottom: 8px;
  padding: 0;
  border: 1px solid rgba(255, 255, 255, 0.24);
  border-radius: 4px;
  color: ${({ theme }) => theme.colors.text.main};
  background-color: ${props => props.theme.colors.levels.sunken};
  font-weight: regular;
  font-size: 12px;

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.levels.elevated};
  }

  > span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding: 2px 4px 2px 12px;
  }

  button {
    color: ${({ theme }) => theme.colors.text.main};
    border-radius: 0;
    font-size: 14px;
    min-width: 10px;
    height: 100%;
    border-bottom-right-radius: 4px;
    border-top-right-radius: 4px;
  }
`;

type Option = BaseOption & {
  // filter preservers the original data.
  filter: Filter;
};

export type Props = {
  // filters is a list of all available filters.
  filters: Filter[];
  // appliedFilters are a list of filters that have been
  // applied to a list of data. Used to render labels list and
  // to update selected items for the select dropdown list on:
  //  - first render (labels from query params if any)
  //  - when labels are clicked from table
  appliedFilters: Filter[];
  // applyFilters applies the filters to the list of data and
  // updates appliedFilters.
  applyFilters(newFilters: Filter[]): void;
  // mb is margin-bottom and is applied to the select button.
  mb?: number;
  // pageSize is number of filters to list per page.
  pageSize?: number;
};
