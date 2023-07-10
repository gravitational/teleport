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

export const MonospacedOutput = styled.pre.attrs({
  'data-scrollbar': 'default',
})`
  background: #161b22;
  color: white;
  padding: 10px 18px;
  margin: 0;
  overflow-x: auto;
  font-family: ${p => p.theme.fonts.mono};
  font-size: 13px;
`;
