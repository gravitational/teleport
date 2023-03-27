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

import Flex from './Flex';
import Box from './../Box';

export default {
  title: 'Design/Flex',
};

export const Basic = () => (
  <Flex gap={5}>
    <Box width={1 / 2} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={1 / 2} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);

export const Wrapped = () => (
  <Flex flexWrap="wrap" gap={2}>
    <Box width={[1, 1 / 2]} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={[1, 1 / 2]} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);

export const Justified = () => (
  <Flex justifyContent="space-around">
    <Box width={1 / 3} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={1 / 3} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);
