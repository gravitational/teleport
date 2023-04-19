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

export const Mark = styled.mark`
  padding: 2px 5px;
  border-radius: 6px;
  font-family: ${props => props.theme.fonts.mono};
  font-size: 12px;
  background-color: ${props =>
    props.light ? '#d3d3d3' : props.theme.colors.spotBackground[2]};
  color: inherit;
`;
