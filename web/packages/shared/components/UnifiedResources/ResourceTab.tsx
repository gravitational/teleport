/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import styled from 'styled-components';

import { Box, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';

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

const TabBox = styled(Box)<{ disabled?: boolean; selected?: boolean }>`
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
  color: ${props =>
    props.disabled
      ? props.theme.colors.text.disabled
      : props.theme.colors.text.main};
  border-bottom: ${props =>
    props.selected ? `2px solid ${props.theme.colors.brand}` : 'transparent'};
`;

const TabText = styled(Text)<{ selected?: boolean }>`
  font-size: ${props => props.theme.fontSizes[2]};
  font-weight: ${props =>
    props.selected
      ? props.theme.fontWeights.bold
      : props.theme.fontWeights.regular};
  line-height: 20px;

  color: ${props =>
    props.selected ? props.theme.colors.brand : props.theme.colors.text.main};
`;

type Props = {
  title: string;
  isSelected: boolean;
  disabled: boolean;
  onClick: () => void;
};
