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

import Icon from 'design/Icon';

export const StyledArrowBtn = styled.button`
  background: none;
  border: none;
  cursor: pointer;

  ${Icon} {
    font-size: 20px;
    transition: all 0.3s;
    opacity: 0.5;
  }

  &:hover,
  &:focus {
    ${Icon} {
      opacity: 1;
    }
  }

  &:disabled {
    cursor: default;
    ${Icon} {
      opacity: 0.1;
    }
  }
`;

export const StyledFetchMoreBtn = styled.button`
  color: ${props => props.theme.colors.link};
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
    color: ${props => props.theme.colors.action.disabled};
    cursor: wait;
  }
`;
