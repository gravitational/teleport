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

import { Flex } from 'design';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

export const LockedFeatureContainer = styled(Flex)`
  flex-wrap: wrap;
  position: relative;
  justify-content: center;
  margin-top: ${p => p.theme.space[4]}px;
  min-width: 224px;
`;

export const LockedFeatureButton = styled(ButtonLockedFeature)`
  position: absolute;
  width: 80%;
  right: 10%;
  bottom: -10px;

  @media screen and (max-width: ${props => props.theme.breakpoints.tablet}px) {
    width: 100%;
    right: 1px;
  } ;
`;
