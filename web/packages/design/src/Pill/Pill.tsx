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

import React from 'react';
import styled from 'styled-components';

import { Cross } from '../Icon';

export function Pill({
  label,
  onDismiss,
}: {
  label: string;
  onDismiss?: (labelName: string) => void;
}) {
  const dismissable = !!onDismiss;
  return (
    <Wrapper dismissable={dismissable}>
      <Label>{label}</Label>
      <Dismiss
        role="button"
        dismissable={dismissable}
        onClick={(e: React.MouseEvent) => {
          e.stopPropagation();
          onDismiss?.(label);
        }}
      >
        <Cross size="small" />
      </Dismiss>
    </Wrapper>
  );
}

const Wrapper = styled.span<{ dismissable?: boolean }>`
  background: ${props => props.theme.colors.spotBackground[1]};
  border-radius: 35px;
  cursor: default;
  display: inline-block;
  padding: ${props => (props.dismissable ? '6px 6px 6px 14px;' : '6px 14px;')};
  white-space: nowrap;
`;

const Label = styled.span`
  display: inline;
`;

const Dismiss = styled.button<{ dismissable?: boolean }>`
  border-color: rgba(0, 0, 0, 0);
  background-color: rgba(0, 0, 0, 0);
  cursor: pointer;
  display: ${props => (props.dismissable ? 'inline-block' : 'none')};
`;
