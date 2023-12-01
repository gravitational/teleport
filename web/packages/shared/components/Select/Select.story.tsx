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

import React from 'react';
import { Flex, Box } from 'design';

import Select, { DarkStyledSelect } from '../Select';

export default {
  title: 'Shared/Select',
};

export const Selects = () => {
  return (
    <Flex>
      <SelectDefault {...props} />
      <SelectDark {...props} />
    </Flex>
  );
};

const props = {
  value: [
    { value: 'admin', label: 'admin' },
    { value: 'testrole', label: 'testrole' },
  ],
  onChange: () => null,
  options: [
    { value: 'Relupba', label: 'Relupba' },
    { value: 'B', label: 'B' },
    { value: 'Pilhibo', label: 'Pilhibo' },
  ],
};

function SelectDefault({ value, onChange, options }) {
  const [selected, setSelected] = React.useState([]);

  return (
    <Flex flexDirection="column" width="330px" mr={5}>
      <Box mb="200px">
        <Select
          value={value}
          onChange={onChange}
          options={options}
          isMulti={true}
        />
      </Box>
      <Box mb="200px">
        <Select
          value={selected}
          onChange={(opt: any) => setSelected(opt)}
          options={options}
          placeholder="Click to select a role"
        />
      </Box>
      <Box>
        <Select
          isDisabled={true}
          value={selected}
          onChange={(opt: any) => setSelected(opt)}
          options={options}
          placeholder="Click to select a role"
        />
      </Box>
    </Flex>
  );
}

function SelectDark({ value, onChange, options }) {
  const [selected, setSelected] = React.useState([]);

  return (
    <Flex flexDirection="column" width="330px" mr={5}>
      <DarkStyledSelect mb="206px">
        <Select
          value={value}
          onChange={onChange}
          options={options}
          isMulti={true}
        />
      </DarkStyledSelect>
      <DarkStyledSelect mb="206px">
        <Select
          value={selected}
          onChange={(opt: any) => setSelected(opt)}
          options={options}
          placeholder="Click to select a role"
        />
      </DarkStyledSelect>
      <DarkStyledSelect>
        <Select
          isDisabled={true}
          value={selected}
          onChange={(opt: any) => setSelected(opt)}
          options={options}
          placeholder="Click to select a role"
        />
      </DarkStyledSelect>
    </Flex>
  );
}
