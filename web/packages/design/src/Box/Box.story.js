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
import Box from './Box';

export default {
  title: 'Design/Box',
};

export const Colors = () => (
  <>
    <Box bg="blue" p={3} color="white">
      Hello
    </Box>
    <Box mt="4" bg="yellow" p={3} color="red">
      Hello
    </Box>
    <Box mt="4" bg="#ffffff" p={3} color="red">
      Hello
    </Box>
  </>
);

export const BorderRadius = () => (
  <>
    <Box borderRadius="16px" color="white" p={5} bg="blue">
      16 Pixel Border Radius
    </Box>
    <Box
      mt={4}
      borderBottomRightRadius={3}
      borderBottomLeftRadius={3}
      color="white"
      p={5}
      bg="blue"
    >
      Border Radius on Bottom Left & Right
    </Box>
    <Box
      mt={4}
      borderTopLeftRadius={16}
      borderTopRightRadius={16}
      color="white"
      p={5}
      bg="blue"
    >
      {`Border Radius on Top Left & Right`}
    </Box>
  </>
);
export const Width = () => (
  <>
    <Box mt={2} p={3} width={1 / 2} color="white" bg="blue">
      Half Width
    </Box>
    <Box mt={2} p={3} width={256} color="white" bg="blue">
      256px width
    </Box>
    <Box mt={2} p={3} width="50vw" color="white" bg="blue">
      50vw width
    </Box>
  </>
);

export const DirectionalPadding = () => (
  <Box p={3}>
    <Box m={2} pt={3} color="white" bg="blue">
      Padding Top
    </Box>
    <Box m={2} pr={4} color="white" bg="blue">
      Padding Right
    </Box>
    <Box m={2} pb={3} color="white" bg="blue">
      Padding Bottom
    </Box>
    <Box m={2} pl={4} color="white" bg="blue">
      Padding Left
    </Box>
    <Box m={2} px={4} color="white" bg="blue">
      Padding X-Axis
    </Box>
    <Box m={2} py={4} color="white" bg="blue">
      Padding Y-Axis
    </Box>
  </Box>
);

export const DirectionalMargin = () => (
  <Box p={3}>
    <Box mt={5} color="white" bg="blue">
      Margin Top
    </Box>
    <Box mr={3} color="white" bg="blue">
      Margin Right
    </Box>
    <Box mb={4} color="white" bg="blue">
      Margin Bottom
    </Box>
    <Box ml={5} color="white" bg="blue">
      Margin Left
    </Box>
    <Box mx={5} color="white" bg="blue">
      Margin X-Axis
    </Box>
    <Box my={5} color="white" bg="blue">
      Margin Y-Axis
    </Box>
  </Box>
);
