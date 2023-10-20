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

import { Card } from 'design';

export const OnboardCard = styled(Card)<{ center: boolean }>`
  width: 600px;
  padding: ${props => props.theme.space[4]}px;
  text-align: ${props => (props.center ? 'center' : 'left')};
  margin: ${props => props.theme.space[3]}px auto
    ${props => props.theme.space[3]}px auto;
  overflow-y: auto;

  @media screen and (max-width: 800px) {
    width: auto;
    margin: 20px;
  }

  @media screen and (max-height: 760px) {
    height: calc(100vh - 250px);
  }
`;
