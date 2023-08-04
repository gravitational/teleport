import Box from 'design/Box';
import { ButtonBorder } from 'design/Button';
import Flex from 'design/Flex';
import * as icons from 'design/Icon';
import React from 'react';
import Select from 'shared/components/Select';
import styled from 'styled-components';
import { SortType } from 'teleport/services/agents';

const filterOptions = [
  { label: 'Application', value: 'app' },
  { label: 'Database', value: 'db' },
  { label: 'Desktop', value: 'windows_desktop' },
  { label: 'Kubernetes', value: 'kube_cluster' },
  { label: 'Server', value: 'node' },
];

const sortOptions = [
  { label: 'Name', value: 'name' },
  { label: 'Type', value: 'kind' },
];

export interface FilterPanelProps {
  sort: SortType;
  setSort: (sort: SortType) => void;
}

export function FilterPanel({ sort, setSort }: FilterPanelProps) {
  const [filter, setFilter] = React.useState(null);
  const [sortMenuAnchor, setSortMenuAnchor] = React.useState(null);

  const sortFieldOption = sortOptions.find(opt => opt.value === sort.fieldName);

  const onFilterChanged = (filter: any) => {
    setFilter(filter);
  };

  const onSortFieldChange = (option: any) => {
    setSort({ ...sort, fieldName: option.value });
  };

  const onSortMenuButtonClicked = event => {
    setSortMenuAnchor(event.currentTarget);
  };

  const onSortMenuClosed = () => {
    setSortMenuAnchor(null);
  };

  const onSortOrderButtonClicked = () => {
    setSort(oppositeSort(sort));
  };

  return (
    <Flex justifyContent="space-between" mb={2}>
      <Box width="300px">
        <Select
          isMulti={true}
          placeholder="Type"
          options={filterOptions}
          value={filter}
          onChange={onFilterChanged}
        />
      </Box>
      <Flex>
        <Box width="100px">
          <SortSelect
            options={sortOptions}
            value={sortFieldOption}
            onChange={onSortFieldChange}
          />
        </Box>
        <SortOrderButton px={3} onClick={onSortOrderButtonClicked}>
          {sort.dir === 'ASC' && <icons.SortAsc />}
          {sort.dir === 'DESC' && <icons.SortDesc />}
        </SortOrderButton>
      </Flex>
    </Flex>
  );
  return null;
}

function oppositeSort(sort: SortType): SortType {
  switch (sort.dir) {
    case 'ASC':
      return { ...sort, dir: 'DESC' };
    case 'DESC':
      return { ...sort, dir: 'ASC' };
    default:
      // Will never happen. Of course.
      return sort;
  }
}

const SortOrderButton = styled(ButtonBorder)`
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
`;

const SortSelect = styled(Select)`
  .react-select__control {
    border-right: none;
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }
  .react-select__dropdown-indicator {
    display: none;
  }
`;
