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

import { ReactNode } from 'react';
import styled from 'styled-components';

import { H3 } from 'design';

export default function TabIcon({ Icon, ...props }: Props) {
  return (
    <StyledTab
      ml="4"
      key={props.title}
      active={props.active}
      onClick={props.onClick}
    >
      <Icon size="medium" />
      {props.title}
    </StyledTab>
  );
}

type Props = {
  active: boolean;
  onClick(): void;
  title: string;
  Icon: (any) => ReactNode;
};

const StyledTab = styled(H3)<{ active?: boolean }>`
  align-items: center;
  display: flex;
  padding: 4px 8px;
  cursor: pointer;
  border-bottom: 4px solid transparent;

  svg {
    margin-right: 8px;
  }

  ${({ active, theme }) =>
    active
      ? `border-bottom: 4px solid ${theme.colors.brand};`
      : `font-weight: 400;`}
`;
