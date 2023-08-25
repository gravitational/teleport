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

import { Box, Flex } from 'design';
import styled from 'styled-components';

export const ConnectorBox = styled(Box)`
  display: flex;
  flex-direction: column;
  width: 320px;
  padding: ${p => p.theme.space[4]}px;
  margin: ${p => p.theme.space[3]}px ${p => p.theme.space[2]}px;
  background: transparent;
  transition: all 0.3s;
  border-radius: ${props => props.theme.radii[2]}px;
  min-height: 190px;
  border: ${props => props.theme.borders[2]}
    ${p => p.theme.colors.spotBackground[0]};

  &:hover,
  &:focus {
    border: ${props => props.theme.borders[2]}
      ${p => p.theme.colors.spotBackground[2]};
    background: ${p => p.theme.colors.spotBackground[0]};
    box-shadow: ${p => p.theme.boxShadow[3]};
    cursor: pointer;
  }

  &:disabled {
    cursor: not-allowed;
    color: inherit;
    font-family: inherit;
    outline: none;
    position: relative;
    text-align: center;
    text-decoration: none;
    opacity: 0.24;
    box-shadow: none;
  }
`;

export const ResponsiveConnector = styled(Flex)`
  position: relative;
  box-shadow: ${p => p.theme.boxShadow[5]};
  width: 240px;
  height: 240px;
  border-radius: ${props => props.theme.radii[2]}px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: ${props => props.theme.space[5]}px;
  @media screen and (max-width: ${props => props.theme.breakpoints.tablet}px) {
    width: 100%;
  }
`;
