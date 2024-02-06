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

import { useState } from 'react';
import { Flex } from 'design';

import Select, { Option } from '../Select';

export default {
  title: 'Shared/Select',
};

const options: Option[] = [
  { value: 'access-role', label: 'access' },
  { value: 'editor-role', label: 'editor' },
  { value: 'auditor-role', label: 'auditor' },
];

export function Selects() {
  const [selectedMulti, setSelectedMulti] = useState(options.slice(0, 2));
  const [selectedSingle, setSelectedSingle] = useState(options[0]);

  return (
    <Flex flexDirection="column" width="330px" gap={10}>
      <Select
        value={selectedMulti}
        onChange={options => setSelectedMulti(options as Option[])}
        options={options}
        placeholder="Click to select a role"
        isMulti={true}
      />
      <Select
        value={selectedMulti}
        onChange={options => setSelectedMulti(options as Option[])}
        options={options}
        placeholder="Click to select a role"
        isMulti={true}
        isDisabled={true}
      />
      <Select
        value={selectedSingle}
        onChange={option => setSelectedSingle(option as Option)}
        options={options}
        placeholder="Click to select a role"
      />
      <Select
        isDisabled={true}
        value={selectedSingle}
        onChange={option => setSelectedSingle(option as Option)}
        options={options}
        placeholder="Click to select a role"
      />
    </Flex>
  );
}
