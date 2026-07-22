/*
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

import { ComponentProps, ElementType, Ref } from 'react';
import styled, { CSSObject } from 'styled-components';

import {
  alignSelf,
  AlignSelfProps,
  color,
  ColorProps,
  space,
  SpaceProps,
} from 'design/system';

import { buttonSizes } from './constants';

type Size = 0 | 1 | 2;

const size = (props: { size: Size }): CSSObject => {
  return buttonSizes[props.size];
};

type ButtonIconProps<E extends ElementType> = ComponentProps<E> &
  SpaceProps &
  ColorProps &
  AlignSelfProps & {
    size?: Size;
    /** If defined, changes the underlying component type. */
    as?: E;
    ref?: Ref<HTMLButtonElement>;
  };

const ButtonIcon = <E extends ElementType = 'button'>({
  children,
  ref,
  css,
  size = 1,
  ...rest
}: ButtonIconProps<E>) => {
  return (
    <StyledButtonIcon ref={ref} css={css} size={size} {...rest}>
      {children}
    </StyledButtonIcon>
  );
};

const StyledButtonIcon = styled.button<{ size: Size }>`
  align-items: center;
  border: none;
  cursor: pointer;
  display: flex;
  outline: none;
  border-radius: 50%;
  overflow: visible;
  justify-content: center;
  text-align: center;
  flex: 0 0 auto;
  background: transparent;
  color: inherit;
  transition: all 0.3s;
  -webkit-font-smoothing: antialiased;

  &:disabled {
    color: ${({ theme }) => theme.colors.text.disabled};
    cursor: default;
  }

  // Using :not(:disabled) instead of :enabled since ButtonIcon can be used with as="a"
  // and :enabled doesn't work with <a> tags.
  &:not(:disabled) {
    &:hover,
    &:focus {
      background: ${({ theme }) => theme.colors.spotBackground[1]};
    }

    &:active {
      background: ${({ theme }) => theme.colors.spotBackground[2]};
    }
  }

  ${size}
  ${space}
  ${color}
  ${alignSelf}
`;
export default ButtonIcon;
