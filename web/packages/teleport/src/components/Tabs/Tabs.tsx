/**
 * Copyright 2022 Gravitational, Inc.
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

import React, { useState } from 'react';
import { Box, Flex } from 'design';

export function Tabs({ tabs }: Props) {
  const [currContentIndex, setCurrContentIndex] = useState(0);
  return (
    <Box>
      <Flex
        css={`
          overflow-x: auto;
          white-space: nowrap;
          // Add background color to scrollbar if it appears.
          // Only applies to chrome and safari as firefox uses
          // overlay scrollbars.
          ::-webkit-scrollbar-track {
            background: rgba(255, 255, 255, 0.05);
          }
        `}
      >
        {tabs.map((tab, index) => {
          const isActive = index === currContentIndex;
          return (
            <Box
              as="button"
              key={index}
              onClick={() => setCurrContentIndex(index)}
              py={3}
              px={4}
              borderTopLeftRadius={2}
              borderTopRightRadius={2}
              bg={isActive ? 'rgba(255, 255, 255, 0.05)' : 'transparent'}
              css={`
                opacity: ${isActive ? 1 : 0.6};
                color: inherit;
                border: none;
                :hover,
                :focus {
                  cursor: pointer;
                  opacity: 1;
                }
              `}
            >
              {tab.title}
            </Box>
          );
        })}
      </Flex>
      <Box
        p={2}
        borderBottomLeftRadius={2}
        borderBottomRightRadius={2}
        bg="rgba(255, 255, 255, 0.05)"
      >
        {tabs[currContentIndex].content}
      </Box>
    </Box>
  );
}

type Tab = {
  // title is the tab title.
  title: string;
  // content is the component to render as children in
  // the tabs container.
  content: JSX.Element;
};

export type Props = {
  tabs: Tab[];
};
