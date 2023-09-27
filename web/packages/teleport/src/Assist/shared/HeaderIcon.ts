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

import { Tooltip } from 'teleport/Assist/shared/Tooltip';

export const HeaderIcon = styled.div`
  border-radius: 7px;
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: 0.2s ease-in-out opacity;
  position: relative;

  ${Tooltip} {
    display: none;
  }

  svg {
    transform: ${p => (p.rotated ? 'rotate(180deg)' : 'none')};
  }

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};

    ${Tooltip} {
      display: block;
    }
  }
`;
