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

export const ListItem = styled.li`
  white-space: nowrap;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  cursor: pointer;
  width: 100%;
  position: relative;
  font-size: 14px;
  padding: 0 16px;
  font-weight: ${props => props.theme.regular};
  font-family: ${props => props.theme.font};
  color: ${props => props.theme.colors.text.primary};
  height: 34px;
  background: inherit;
  border: none;
  border-radius: 4px;

  background: ${props => (props.isActive ? 'rgba(255, 255, 255, 0.05)' : null)};

  &:focus,
  &:hover {
    background: rgba(255, 255, 255, 0.05);
    color: ${props => props.theme.colors.text.contrast};
  }
`;
