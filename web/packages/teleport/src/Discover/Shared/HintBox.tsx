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
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';

import * as Icons from 'design/Icon';

import { TextIcon } from 'teleport/Discover/Shared/Text';

const HintBoxContainer = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid ${props => props.theme.colors.warning.main};
`;

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
  border: 2px solid ${props => props.theme.colors.success};
  display: flex;
  align-items: center;
`;

interface HintBoxProps {
  header: string;
}

export function HintBox(props: React.PropsWithChildren<HintBoxProps>) {
  return (
    <HintBoxContainer>
      <Text color="warning.main">
        <Flex alignItems="center" mb={2}>
          <TextIcon
            color="warning.main"
            css={`
              white-space: pre;
            `}
          >
            <Icons.Warning fontSize={4} color="warning.main" />
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
        <Icons.CircleCheck fontSize={4} color="success" />
      </TextIcon>
      {props.children}
    </SuccessInfo>
  );
}
