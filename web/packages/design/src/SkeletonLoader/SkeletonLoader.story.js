/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { Box, Flex } from 'design';

import { SkeletonLoader } from './SkeletonLoader';

export default {
  title: 'SkeletonLoader',
};

export const Cards = () => {
  return (
    <Flex gap={2} flexWrap="wrap">
      {new Array(10).fill(null).map((_, i) => (
        <LoadingCard key={i} />
      ))}
    </Flex>
  );
};

const LoadingCard = () => {
  return (
    <Box p={2} width="300px">
      <Flex gap={2}>
        <Box width="45px" height="45px">
          <SkeletonLoader />
        </Box>
        <Box flex={1}>
          <Box height="20px" mb={2}>
            <SkeletonLoader />
          </Box>
          <Box height="12px">
            <SkeletonLoader />
          </Box>
        </Box>
      </Flex>
    </Box>
  );
};
