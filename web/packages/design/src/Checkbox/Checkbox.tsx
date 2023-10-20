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

import styled from 'styled-components';

import { Flex } from 'design';
import { space } from 'design/system';

export const CheckboxWrapper = styled(Flex)`
  padding: 8px;
  margin-bottom: 4px;
  width: 300px;
  align-items: center;
  border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  border-radius: 8px;

  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;

export const CheckboxInput = styled.input`
  margin-right: 10px;
  accent-color: ${props => props.theme.colors.brand};

  &:hover {
    cursor: pointer;
  }

  ${space}
`;

// TODO (avatus): Make this the default checkbox
export const StyledCheckbox = styled.input.attrs({ type: 'checkbox' })`
  // reset the appearance so we can style the background
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: ${props => props.theme.radii[1]}px;
  background: transparent;
  position: relative;

  &:checked {
    border: 1px solid ${props => props.theme.colors.brand};
    background-color: ${props => props.theme.colors.brand};
  }

  &:hover {
    cursor: pointer;
  }

  &::before {
    content: '';
    display: block;
  }

  &:checked::before {
    content: '✓';
    color: ${props => props.theme.colors.levels.deep};
    position: absolute;
    right: 1px;
  }
`;
