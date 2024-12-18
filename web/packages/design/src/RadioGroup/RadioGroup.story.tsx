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

import { Box, Flex } from 'design';

import { RadioGroup } from './RadioGroup';

export default {
  title: 'Design/RadioGroup',
};

export const Default = () => {
  return (
    <Flex flexDirection="column">
      <Box>
        <h4>String options</h4>
        <RadioGroup
          name="example1"
          options={[
            'First option',
            'Second option',
            'Third option',
            'Fourth option',
          ]}
        />
      </Box>
      <Box>
        <h4>With value set</h4>
        <RadioGroup
          name="example2"
          options={['First option', 'Second option', 'Third option']}
          value={'Second option'}
        />
      </Box>
      <Box>
        <h4>Object options with value set</h4>
        <RadioGroup
          name="example3"
          options={[
            { value: '1', label: <span css={'color: red'}>First option</span> },
            {
              value: '2',
              label: <span css={'color: navajowhite'}>Second option</span>,
            },
          ]}
          value={'1'}
        />
      </Box>
      <Box>
        <h4>With a disabled value</h4>
        <RadioGroup
          name="example4"
          options={[
            { value: '1', label: 'First option' },
            {
              value: '2',
              label: 'Disabled option',
              disabled: true,
            },
          ]}
        />
      </Box>
      <Box>
        <h4>With a helper text</h4>
        <RadioGroup
          name="example5"
          options={[
            {
              value: '1',
              label: 'First option',
              helperText: 'First option helper text',
            },
            {
              value: '2',
              label: 'Second option',
              helperText: 'Second option helper text',
            },
          ]}
        />
      </Box>
      <Box>
        <h4>Small</h4>
        <RadioGroup
          name="example6"
          size="small"
          options={['First option', 'Second option']}
        />
      </Box>
    </Flex>
  );
};
