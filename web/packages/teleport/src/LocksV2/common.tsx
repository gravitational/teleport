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

import styled from 'styled-components';
import { Trash } from 'design/Icon';

export const TrashButton = styled(Trash)`
  padding: 8px;
  font-size: 13px;
  border-radius: 2px;
  cursor: pointer;
  background-color: ${({ theme }) => theme.colors.buttons.trashButton.default};
  :hover {
    background-color: ${({ theme }) => theme.colors.buttons.trashButton.hover};
  }
`;
