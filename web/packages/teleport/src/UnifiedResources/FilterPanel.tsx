import Box from 'design/Box';
import { ButtonBorder } from 'design/Button';
import Flex from 'design/Flex';
import Menu, { MenuItem } from 'design/Menu';
import React from 'react';
import Select from 'shared/components/Select';

const filterOptions = [
  { label: 'Application', value: 'app' },
  { label: 'Database', value: 'db' },
  { label: 'Desktop', value: 'windows_desktop' },
  { label: 'Kubernetes', value: 'kube_cluster' },
  { label: 'Server', value: 'node' },
];

const sortOptions = [];

export function FilterPanel() {
  const [filter, setFilter] = React.useState(null);
  const [sortMenuAnchor, setSortMenuAnchor] = React.useState(null);

  const onFilterChanged = (filter: any) => {
    setFilter(filter);
  };

  const onSortMenuButtonClicked = event => {
    setSortMenuAnchor(event.currentTarget);
  };

  const onSortMenuClosed = () => {
    setSortMenuAnchor(null);
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
      <SortSelect></SortSelect>
      {/* <ButtonBorder ref={sortMenuAnchor} onClick={onSortMenuButtonClicked}>
        Name
      </ButtonBorder>
      <Menu
        anchorEl={sortMenuAnchor}
        open={!!sortMenuAnchor}
        onClose={onSortMenuClosed}
      >
        <MenuItem>Name</MenuItem>
      </Menu> */}
    </Flex>
  );
  return null;
}

const SortSelect = styled(Select)``;
