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

export const StepTitle = styled.div`
  display: flex;
  align-items: center;
`;

export const StepsContainer = styled.div<{ active: boolean }>`
  display: flex;
  flex-direction: column;
  color: ${p => (p.active ? 'inherit' : p.theme.colors.text.slightlyMuted)};
  margin-right: ${p => p.theme.space[5]}px;
  position: relative;

  &:after {
    position: absolute;
    content: '';
    width: 16px;
    background: ${({ theme }) => theme.colors.brand};
    height: 1px;
    top: 50%;
    transform: translate(0, -50%);
    right: -25px;
  }

  &:last-of-type {
    &:after {
      display: none;
    }
  }
`;
