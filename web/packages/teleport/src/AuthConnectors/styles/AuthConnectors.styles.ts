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
import { Box, ButtonPrimary, Text } from 'design';

import { FeatureHeader } from 'teleport/components/Layout';

export const ResponsiveFeatureHeader = styled(FeatureHeader)`
  justify-content: space-between;

  @media screen and (max-width: 800px) {
    flex-direction: column;
    height: auto;
    gap: 10px;
    margin: 0 0 10px 0;
    padding: 0 0 10px 0;
    align-items: start;
  }
`;

export const MobileDescription = styled(Text)`
  margin-bottom: ${p => p.theme.space[3]}px;
  @media screen and (min-width: 800px) {
    display: none;
  } ;
`;

export const DesktopDescription = styled(Box)`
  margin-left: ${p => p.theme.space[4]}px;
  width: 240px;
  color: ${p => p.theme.colors.text.main};
  flex-shrink: 0;
  @media screen and (max-width: 800px) {
    display: none;
  } ;
`;

export const ResponsiveAddButton = styled(ButtonPrimary)`
  width: 240px;
  @media screen and (max-width: 800px) {
    width: 100%;
  } ;
`;
