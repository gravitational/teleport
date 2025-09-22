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

import { Fragment, useState, type JSX } from 'react';

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
          }
        `}
      >
        {tabs.map((tab, index) => {
          const isActive = index === currContentIndex;
          return (
            <Fragment key={index}>
              {isActive ? (
                <Box
                  as="button"
                  onClick={() => setCurrContentIndex(index)}
                  py={3}
                  px={4}
                  borderTopLeftRadius={2}
                  borderTopRightRadius={2}
                  css={`
                    color: inherit;
                    border: none;
                    background: ${props =>
                      props.theme.colors.spotBackground[0]};
                    &:hover,
                    &:focus {
                      cursor: pointer;
                      opacity: 1;
                    }
                  `}
                >
                  {tab.title}
                </Box>
              ) : (
                <Box
                  as="button"
                  onClick={() => setCurrContentIndex(index)}
                  py={3}
                  px={4}
                  borderTopLeftRadius={2}
                  borderTopRightRadius={2}
                  css={`
                    color: inherit;
                    border: none;
                    background: transparent;
                    &:hover,
                    &:focus {
                      cursor: pointer;
                      opacity: 1;
                    }
                  `}
                >
                  {tab.title}
                </Box>
              )}
            </Fragment>
          );
        })}
      </Flex>
      <Box
        p={2}
        borderBottomLeftRadius={2}
        borderBottomRightRadius={2}
        css={`
          background: ${props => props.theme.colors.spotBackground[0]};
        `}
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
