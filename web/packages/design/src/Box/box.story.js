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
import Box from './Box';

storiesOf('Desigin/Box', module)
  .addDecorator(withInfo)
  .add('Layout component', () => (
    <Box color="white" bg="blue">
      this is a simple div
    </Box>
  ))
  .add('Padding', () => (
    <Box bg="blue" p={3}>
      Hello
    </Box>
  ))
  .add('Margin', () => (
    <Box bg="blue" m={4}>
      Hello
    </Box>
  ))
  .add('Color', () => (
    <Box bg="blue" p={3} color='white'>
      Hello
    </Box>
  ))
  .add('Background Color', () => (
    <Box bg="blue" p={3} color='white'>
      Hello
    </Box>
  ))
  .add('Border Radius', () => (
    <div>
      <Box
        borderRadius="16px"
        color='white'
        p={5}
        bg='blue'>
        16 Pixel Border Radius
      </Box>
      <Box
        mt={4}
        borderBottomRightRadius={3}
        borderBottomLeftRadius={3}
        color='white'
        p={5}
        bg='blue'>
        Border Radius on Bottom Left & Right
      </Box>
      <Box
        mt={4}
        borderTopLeftRadius={16}
        borderTopRightRadius={16}
        color='white'
        p={5}
        bg='blue'>
        Border Radius on Top Left & Right
      </Box>
    </div>
  ))
  .add('Width', () => (
    <Box
      p={3}
      width={1 / 2}
      color='white'
      bg='blue'>
      Half Width
    </Box>
  ))
  .add('Pixel Width', () => (
    <Box
      p={3}
      width={256}
      color='white'
      bg='blue'>
      256px width
    </Box>
  ))
  .add('VW Width', () => (
    <Box
      p={3}
      width='50vw'
      color='white'
      bg='blue'>
      50vw width
    </Box>
  ))
  .add('Directional Padding', () => (
    <Box p={3}>
      <Box m={2} pt={3} color='white' bg='blue'>Padding Top</Box>
      <Box m={2} pr={4} color='white' bg='blue'>Padding Right</Box>
      <Box m={2} pb={3} color='white' bg='blue'>Padding Bottom</Box>
      <Box m={2} pl={4} color='white' bg='blue'>Padding Left</Box>
      <Box m={2} px={4} color='white' bg='blue'>Padding X-Axis</Box>
      <Box m={2} py={4} color='white' bg='blue'>Padding Y-Axis</Box>
    </Box>
  ))
  .add('Directional Margin', () => (
    <Box p={3}>
      <Box mt={5} color='white' bg='blue'>Margin Top</Box>
      <Box mr={3} color='white' bg='blue'>Margin Right</Box>
      <Box mb={4} color='white' bg='blue'>Margin Bottom</Box>
      <Box ml={5} color='white' bg='blue'>Margin Left</Box>
      <Box mx={5} color='white' bg='blue'>Margin X-Axis</Box>
      <Box my={5} color='white' bg='blue'>Margin Y-Axis</Box>
    </Box>
  ));