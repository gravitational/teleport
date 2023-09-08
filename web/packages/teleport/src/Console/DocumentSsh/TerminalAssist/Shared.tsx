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

export const Key = styled.div`
  line-height: 1;
  background: ${p => p.theme.colors.spotBackground[1]};
  padding: 2px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.space[1]}px;
  font-weight: 700;
  color: ${p => p.theme.colors.text.muted};
`;

export const KeyShortcut = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  opacity: 0.5;
  font-size: 12px;
  pointer-events: none;
  user-select: none;
  transition: opacity 0.2s ease-in-out;
`;
