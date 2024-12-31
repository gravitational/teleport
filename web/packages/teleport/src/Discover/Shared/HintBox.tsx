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

import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

import { TextIcon } from 'teleport/Discover/Shared/Text';

const HintBoxContainer = styled(Box).attrs<{ maxWidth?: string }>(props => ({
  maxWidth: props.maxWidth,
}))`
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid ${props => props.theme.colors.warning.main};
`;

// TODO(bl-nero): Migrate this component to an info or neutral alert box.
export const WaitingInfo = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid ${props => props.theme.colors.text.muted};
  display: flex;
  align-items: center;
`;

export const SuccessInfo = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid ${props => props.theme.colors.success.main};
  display: flex;
  align-items: center;
`;

interface HintBoxProps {
  header: string;
  maxWidth?: string;
}

export function HintBox(props: React.PropsWithChildren<HintBoxProps>) {
  return (
    <HintBoxContainer maxWidth={props.maxWidth || '1000px'}>
      <Text color="warning.main">
        <Flex alignItems="center" mb={2}>
          <TextIcon
            color="warning.main"
            css={`
              white-space: pre;
            `}
          >
            <Icons.Warning size="small" color="warning.main" mr={1} />
          </TextIcon>
          {props.header}
        </Flex>
      </Text>

      {props.children}
    </HintBoxContainer>
  );
}

export function SuccessBox(props: { children: React.ReactNode }) {
  return (
    <SuccessInfo>
      <TextIcon
        css={`
          white-space: pre;
        `}
      >
        <Icons.CircleCheck size="medium" color="success.main" />
      </TextIcon>
      <Box>{props.children}</Box>
    </SuccessInfo>
  );
}
