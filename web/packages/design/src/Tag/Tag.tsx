/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { type HTMLAttributes, type ReactNode } from 'react';
import styled, { css } from 'styled-components';

import { Cross } from 'design/Icon';
import { space, type SpaceProps } from 'design/system';

import { pillBase } from '../pillStyles';

export type TagVariant = 'subtle' | 'outline';

export interface TagProps
  extends SpaceProps, Omit<HTMLAttributes<HTMLSpanElement>, 'color'> {
  variant?: TagVariant;
  // When onDismiss is provided, a dismiss button is rendered and this function
  // is called when clicked.
  onDismiss?: () => void;
  children: ReactNode;
}

interface StyledTagProps extends SpaceProps {
  $variant: TagVariant;
  $interactive: boolean;
}

const StyledTag = styled.span<StyledTagProps>`
  ${pillBase}
  cursor: default;
  color: ${p => p.theme.colors.text.main};
  border: 1px solid ${p => p.theme.colors.text.slightlyMuted};

  background: ${p =>
    p.$variant === 'subtle'
      ? p.theme.colors.interactive.tonal.neutral[1]
      : 'transparent'};

  ${p =>
    p.$interactive &&
    css`
      cursor: pointer;

      &:hover {
        background: ${p.theme.colors.interactive.tonal.neutral[2]};
      }
    `}

  ${space}
`;

const DismissButton = styled.button`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin: 0;
  border: none;
  background: none;
  cursor: pointer;
  color: ${p => p.theme.colors.text.muted};
  border-radius: 50%;

  &:hover {
    color: ${p => p.theme.colors.text.main};
  }
`;

export function Tag({
  variant = 'subtle',
  onDismiss,
  onClick,
  children,
  ...rest
}: TagProps) {
  return (
    <StyledTag
      $variant={variant}
      $interactive={!!onClick}
      onClick={onClick}
      {...rest}
    >
      {children}
      {onDismiss && (
        <DismissButton
          type="button"
          onClick={e => {
            e.stopPropagation();
            onDismiss();
          }}
          aria-label="Remove"
        >
          <Cross size="small" />
        </DismissButton>
      )}
    </StyledTag>
  );
}
