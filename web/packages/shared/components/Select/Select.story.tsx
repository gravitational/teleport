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
