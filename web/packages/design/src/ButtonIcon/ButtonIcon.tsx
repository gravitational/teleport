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

import styled, { CSSProp } from 'styled-components';

import { alignSelf, color, space } from 'design/system';

import { buttonSizes } from './constants';

type Props<E extends React.ElementType> = React.ComponentPropsWithoutRef<E> & {
  size?: number;
  css?: CSSProp;
  setRef?: React.ForwardedRef<HTMLButtonElement>;
  as?: E;
};

const ButtonIcon = <E extends React.ElementType = 'button'>(
  props: Props<E>
) => {
  const { children, setRef, css, size = 1, ...rest } = props;
  return (
    <StyledButtonIcon {...rest} ref={setRef} css={css} $size={size}>
      {children}
    </StyledButtonIcon>
  );
};

const StyledButtonIcon = styled.button<{ $size: keyof typeof buttonSizes }>`
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
      background: ${({ theme }) => theme.colors.interactive.tonal.neutral[1]};
    }

    &:active {
      background: ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
    }
  }

  ${({ $size }) => buttonSizes[$size] ?? buttonSizes[1]}
  ${space}
  ${color}
  ${alignSelf}
`;
export default ButtonIcon;
