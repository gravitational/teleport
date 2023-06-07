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

export const TopBarButton = styled.button`
  display: flex;
  font-family: inherit;
  background: inherit;
  cursor: pointer;
  align-items: center;
  color: ${props => props.theme.colors.text.main};
  flex-direction: row;
  height: 100%;
  border-radius: 4px;
  border-width: 1px;
  border-style: solid;
  border-color: ${props =>
    props.isOpened ? props.theme.colors.buttons.border.border : 'transparent'};

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;
