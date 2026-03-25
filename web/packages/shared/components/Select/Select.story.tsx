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

import { Meta } from '@storybook/react-vite';
import { useState } from 'react';

import { Box, Flex, H3, H4 } from 'design';

import Select, { Option } from '../Select';

type StoryProps = {
  readOnly?: boolean;
  isDisabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared/Select',
  component: Controls,
  argTypes: {
    readOnly: {
      control: { type: 'boolean' },
    },
    isDisabled: {
      control: { type: 'boolean' },
    },
  },
};
export default meta;

const options: Option[] = [
  { value: 'access-role', label: 'access' },
  { value: 'editor-role', label: 'editor' },
  { value: 'auditor-role', label: 'auditor' },
];

export function Controls(props: StoryProps) {
  const [selectedMulti, setSelectedMulti] = useState<readonly Option[]>(
    options.slice(0, 2)
  );
  const [selectedSingle, setSelectedSingle] = useState(options[0]);

  return (
    <>
      <Flex flexDirection="column" width="330px" gap={3} mb={3}>
        <Box>
          <H3>Multi</H3>
          <Select
            value={selectedMulti}
            onChange={options => setSelectedMulti(options)}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
        <Box>
          <H3>Multi, clearable</H3>
          <Select
            value={selectedMulti}
            onChange={options => setSelectedMulti(options)}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isClearable
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
        <Box>
          <H3>Multi, empty</H3>
          <Select
            defaultValue={[]}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
        <Box>
          <H3>Single</H3>
          <Select
            value={selectedSingle}
            onChange={option => setSelectedSingle(option)}
            options={options}
            placeholder="Click to select a role"
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
        <Box>
          <H3>Single, empty</H3>
          <Select
            options={options}
            placeholder="Click to select a role"
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
        <Box>
          <H3>Error</H3>
          <Select
            value={selectedSingle}
            onChange={option => setSelectedSingle(option)}
            options={options}
            placeholder="Click to select a role"
            hasError
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Box>
      </Flex>

      <Box>
        <H3>Sizes</H3>
      </Box>
      <Flex gap={4} mb={4}>
        <Flex flex="1" flexDirection="column" gap={3} mt={3}>
          <H4>Large</H4>
          <Select
            size="large"
            value={selectedSingle}
            onChange={option => setSelectedSingle(option)}
            options={options}
            placeholder="Click to select a role"
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
          <Select
            size="large"
            value={selectedMulti}
            onChange={options => setSelectedMulti(options)}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isClearable={true}
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Flex>
        <Flex flex="1" flexDirection="column" gap={3} mt={3}>
          <H4>Medium</H4>
          <Select
            size="medium"
            value={selectedSingle}
            onChange={option => setSelectedSingle(option)}
            options={options}
            placeholder="Click to select a role"
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
          <Select
            size="medium"
            value={selectedMulti}
            onChange={options => setSelectedMulti(options)}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isClearable={true}
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Flex>
        <Flex flex="1" flexDirection="column" gap={3} mt={3}>
          <H4>Small</H4>
          <Select
            size="small"
            value={selectedSingle}
            onChange={option => setSelectedSingle(option)}
            options={options}
            placeholder="Click to select a role"
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
          <Select
            size="small"
            value={selectedMulti}
            onChange={options => setSelectedMulti(options)}
            options={options}
            placeholder="Click to select a role"
            isMulti={true}
            isClearable={true}
            isDisabled={props.isDisabled}
            readOnly={props.readOnly}
          />
        </Flex>
      </Flex>
    </>
  );
}
