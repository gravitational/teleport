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

import React, { ReactNode } from 'react';
import styled from 'styled-components';

export function Toggle({
  isToggled,
  onToggle,
  children,
  disabled,
  className,
}: {
  isToggled: boolean;
  onToggle: () => void;
  children?: ReactNode;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <StyledWrapper disabled={disabled} className={className}>
      <StyledInput
        checked={isToggled}
        onChange={onToggle}
        disabled={disabled}
      />
      <StyledSlider />
      {children}
    </StyledWrapper>
  );
}

const StyledWrapper = styled.label`
  position: relative;
  display: flex;
  align-items: center;
  cursor: pointer;

  &[disabled] {
    cursor: default;
  }
`;

const StyledSlider = styled.div`
  width: 32px;
  height: 12px;
  border-radius: 12px;
  background: ${props => props.theme.colors.levels.surface};
  cursor: inherit;
  flex-shrink: 0;

  &:before {
    content: '';
    position: absolute;
    top: 50%;
    transform: translate(0, -50%);
    width: 16px;
    height: 16px;
    border-radius: 16px;
    background: ${props => props.theme.colors.brand};
  }
`;

const StyledInput = styled.input.attrs({ type: 'checkbox' })`
  opacity: 0;
  position: absolute;
  cursor: inherit;

  &:checked + ${StyledSlider} {
    background: ${props => props.theme.colors.spotBackground[1]};

    &:before {
      transform: translate(16px, -50%);
    }
  }

  &:disabled + ${StyledSlider} {
    background: ${props => props.theme.colors.levels.surface};

    &:before {
      background: ${props => props.theme.colors.grey[700]};
    }
  }
`;
