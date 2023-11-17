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

import { HoverTooltip } from 'shared/components/ToolTip';

import { PINNING_NOT_SUPPORTED_MESSAGE } from './UnifiedResources';

export const ResourceTab = ({
  title,
  disabled,
  isSelected,
  onClick,
}: Props) => {
  const handleClick = () => {
    if (!disabled) {
      onClick();
    }
  };

  const $tab = (
    <TabBox disabled={disabled} onClick={handleClick} selected={isSelected}>
      <TabText selected={isSelected}>{title}</TabText>
    </TabBox>
  );

  if (disabled) {
    return (
      <HoverTooltip tipContent={PINNING_NOT_SUPPORTED_MESSAGE}>
        {$tab}
      </HoverTooltip>
    );
  }
  return $tab;
};

const TabBox = styled(Box)`
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
  color: ${props =>
    props.disabled
      ? props.theme.colors.text.disabled
      : props.theme.colors.text.main};
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
  isSelected: boolean;
  disabled: boolean;
  onClick: () => void;
};
