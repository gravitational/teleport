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

import React from 'react';
import styled from 'styled-components';
import { Box, Text } from 'design';

import { UnifiedTabPreference } from 'teleport/services/userPreferences/types';

export const ResourceTab = ({ title, value, isSelected, onChange }: Props) => {
  const selectTab = () => {
    onChange(value);
  };

  return (
    <TabBox selected={isSelected} onClick={selectTab}>
      <TabText selected={isSelected}>{title}</TabText>
    </TabBox>
  );
};

const TabBox = styled(Box)`
  cursor: pointer;
  border-bottom: ${props =>
    props.selected ? `2px solid ${props.theme.colors.brand}` : 'transparent'};
`;

const TabText = styled(Text)`
  font-size: ${props => props.theme.fontSizes[2]};
  font-weight: ${props =>
    props.selected
      ? props.theme.fontWeights.bold
      : props.theme.fontWeights.regular};
  line-height: 20px;

  color: ${props =>
    props.selected ? props.theme.colors.brand : props.theme.colors.main};
`;

type Props = {
  title: string;
  value: UnifiedTabPreference;
  isSelected: boolean;
  onChange: (value: UnifiedTabPreference) => void;
};
