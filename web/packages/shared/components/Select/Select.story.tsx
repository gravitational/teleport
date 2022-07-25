/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
    <Flex flexDirection="column" width="330px">
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
