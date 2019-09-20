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
import { storiesOf } from '@storybook/react'
import { withInfo } from '@storybook/addon-info'
import Flex from './Flex';
import Box from './../Box';

storiesOf('Design/Flex', module)
  .addDecorator(withInfo)
  .add('Basic', () => (
    <Flex align='center'>
      <Box width={1 / 2} p={3} m={4} color='white' bg='blue'>Flex</Box>
      <Box width={1 / 2} p={1} m={2} color='white' bg='green'>Box</Box>
    </Flex>
  ))
  .add('Wrap', () => (
    <Flex flexWrap="wrap">
      <Box width={[1, 1 / 2]} p={3} color='white' bg='blue'>Flex</Box>
      <Box width={[1, 1 / 2]} p={1} color='white' bg='green'>Wrap</Box>
    </Flex>
  ))
  .add('Justify', () => (
    <Flex justifyContent='space-around'>
      <Box width={1 / 3} p={2} color='white' bg='blue'>Flex</Box>
      <Box width={1 / 3} p={2} color='white' bg='green'>Justify</Box>
    </Flex>
  ));
