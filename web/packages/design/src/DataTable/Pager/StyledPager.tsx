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

import { ButtonIcon } from 'design';
import Icon from 'design/Icon';

export const StyledArrowBtn = styled(ButtonIcon)`
  ${Icon} {
    font-size: 20px;
  }
  ${Icon}:before {
    // arrow icons have some padding that makes them look slightly off-center, padding compensates it
    padding-left: 1px;
  }
`;

export const StyledFetchMoreBtn = styled.button`
  color: ${props => props.theme.colors.buttons.link.default};
  background: none;
  text-decoration: underline;
  text-transform: none;
  outline: none;
  border: none;
  font-weight: bold;
  line-height: 0;
  font-size: 12px;

  &:hover,
  &:focus {
    cursor: pointer;
  }

  &:disabled {
    color: ${props => props.theme.colors.text.disabled};
    cursor: wait;
  }
`;
