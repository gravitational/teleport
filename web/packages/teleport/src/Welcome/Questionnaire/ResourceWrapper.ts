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

import styled from 'styled-components';
import { Flex } from 'design';

export const ResourceWrapper = styled(Flex)`
  flex-direction: column;
  height: 100%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: 12px 0;
  gap: 16px;
  border-radius: ${props => props.theme.radii[2]}px;

  &:hover {
    background-color: ${props => props.theme.colors.spotBackground[0]};
  }

  border: ${({ isSelected, invalid, theme }) => {
    if (isSelected) {
      return `1px solid ${theme.colors.brand}`;
    }
    if (invalid) {
      return `1px solid ${theme.colors.error.main}`;
    }
    return `1px solid ${theme.colors.levels.elevated}`;
  }};
`;
