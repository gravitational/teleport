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

import { Box } from 'design';
import styled from 'styled-components';

export const ConnectorBox = styled(Box)`
  min-width: 336px;
  padding: ${p => p.theme.space[4]}px;
  margin: ${p => p.theme.space[3]}px ${p => p.theme.space[2]}px;
  background: transparent;
  display: flex;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  min-height: 190px;
  border: 2px solid ;
  &:hover, &:focus {
    border: 2px solid ${p => p.theme.colors.spotBackground[2]};
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
  &:disabled {
    opacity: .24;
    box-shadow: none;
  }
`;
