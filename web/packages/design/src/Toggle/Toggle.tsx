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
  size = 'small',
}: {
  isToggled: boolean;
  onToggle: () => void;
  children?: ReactNode;
  disabled?: boolean;
  className?: string;
  size?: 'small' | 'large';
}) {
  return (
    <StyledWrapper disabled={disabled} className={className}>
      <StyledInput
        checked={isToggled}
        onChange={onToggle}
        disabled={disabled}
        size={size}
        data-testid="toggle"
      />
      <StyledSlider size={size} />
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

const size = props => {
  switch (props.size) {
    case 'large':
      return {
        track: {
          width: 40,
          height: 20,
        },
        circle: {
          width: 14,
          height: 14,
          transform: 'translate(3px, -50%)',
        },
        translate: 'translate(23px, -50%)',
      };
    default:
      // small
      return {
        track: {
          width: 32,
          height: 16,
        },
        circle: {
          width: 12,
          height: 12,
          transform: 'translate(2px, -50%)',
        },
        translate: 'translate(18px, -50%)',
      };
  }
};

const StyledSlider = styled.div`
  // the slider 'track'
  ${props => size(props).track};
  border-radius: 10px;
  cursor: inherit;
  flex-shrink: 0;
  background: ${props => props.theme.colors.buttons.secondary.default};
  transition: background 0.15s ease-in-out;

  &:hover {
    background: ${props => props.theme.colors.buttons.secondary.hover};
  }

  &:active {
    background: ${props => props.theme.colors.buttons.secondary.active};
  }

  // the slider 'circle'
  &:before {
    content: '';
    position: absolute;
    top: 50%;
    ${props => size(props).circle};
    border-radius: 14px;
    background: ${props => props.theme.colors.interactionHandle};
    box-shadow: ${props => props.theme.boxShadow[0]};
    transition: transform 0.05s ease-in;
  }
`;

const StyledInput = styled.input.attrs({ type: 'checkbox' })`
  opacity: 0;
  position: absolute;
  cursor: inherit;
  z-index: -1;

  &:checked + ${StyledSlider} {
    &:before {
      transform: ${props => size(props).translate};
    }
  }

  &:enabled:checked + ${StyledSlider} {
    background: ${props => props.theme.colors.success.main};

    &:hover {
      background: ${props => props.theme.colors.success.hover};
    }

    &:active {
      background: ${props => props.theme.colors.success.active};
    }
  }

  &:disabled + ${StyledSlider} {
    background: ${props => props.theme.colors.spotBackground[0]};

    &:before {
      opacity: 0.36;
      box-shadow: none;
    }
  }

  &:disabled:checked + ${StyledSlider} {
    background: ${props => props.theme.colors.interactive.tonal.success[2]};
  }
`;
