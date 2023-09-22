/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { Box, Flex } from 'design';

import { RadioGroup } from './RadioGroup';

export default {
  title: 'Design/RadioGroup',
};

export const Default = () => {
  return (
    <Flex direction="row">
      <Box mr={6}>
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
      <Box mr={6}>
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
    </Flex>
  );
};
