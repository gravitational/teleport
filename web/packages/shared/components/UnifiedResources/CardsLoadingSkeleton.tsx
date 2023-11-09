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

import React, { useState, useEffect } from 'react';
import { Flex, Box } from 'design';
import { ShimmerBox } from 'design/ShimmerBox';

type LoadingCardProps = {
  delay?: 'none' | 'short' | 'long';
};

export function LoadingCard({ delay = 'none' }: LoadingCardProps) {
  const [canDisplay, setCanDisplay] = useState(false);

  useEffect(() => {
    const displayTimeout = setTimeout(() => {
      setCanDisplay(true);
    }, DelayValueMap[delay]);
    return () => {
      clearTimeout(displayTimeout);
    };
  }, []);

  if (!canDisplay) {
    return null;
  }

  return (
    <Flex gap={2} alignItems="start" height="106px" p={3}>
      {/* Checkbox */}
      <ShimmerBox height="16px" width="16px" />
      {/* Image */}
      <ShimmerBox height="45px" width="45px" />
      <Box flex={1}>
        <Flex gap={2} mb={2} justifyContent="space-between">
          {/* Name */}
          <ShimmerBox
            height="24px"
            css={`
              flex-basis: ${randomNum(100, 30)}%;
            `}
          />
          {/* Action button */}
          <ShimmerBox height="24px" width="90px" />
        </Flex>
        {/* Description */}
        <ShimmerBox height="16px" width={`${randomNum(90, 40)}%`} mb={2} />
        {/* Labels */}
        <Flex gap={2}>
          {new Array(randomNum(4, 0)).fill(null).map((_, i) => (
            <ShimmerBox key={i} height="16px" width="60px" />
          ))}
        </Flex>
      </Box>
    </Flex>
  );
}

const DelayValueMap = {
  none: 0,
  short: 400, // 0.4s;
  long: 600, // 0.6s;
};

function randomNum(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}
