/**
 * Copyright 2023 Gravitational, Inc.
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

import React from 'react';
import { Flex, Text, Box } from 'design';
import styled from 'styled-components';

export const IntegrationTile = styled(Flex)`
  color: inherit;
  text-decoration: none;
  flex-direction: column;
  align-items: center;
  position: relative;
  border-radius: 4px;
  height: 170px;
  width: 170px;
  background-color: ${({ theme }) => theme.colors.buttons.secondary.default};
  text-align: center;
  cursor: pointer;

  ${props => {
    const pointerEvents = props.disabled ? 'none' : null;
    if (props.$exists) {
      return { pointerEvents };
    }

    return `
    opacity: ${props.disabled ? '0.45' : '1'};
    &:hover {
      background-color: ${props.theme.colors.buttons.secondary.hover};
    }
    `;
  }}
`;

export const NoCodeIntegrationDescription = () => (
  <Box mb={3}>
    <Text fontWeight="bold" typography="h4">
      No-Code Integrations
    </Text>
    <Text typography="body1">
      Hosted Integrations eliminate the setup work so you can quickly connect
      applications to Teleport for alerting and other useful functions. This
      list is short for now, but it will grow with time!
    </Text>
  </Box>
);
