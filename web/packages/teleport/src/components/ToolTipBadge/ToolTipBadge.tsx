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

import React, { PropsWithChildren } from 'react';

import { Box } from 'design';
import { BaseTooltip } from 'design/Tooltip/shared';

type Props = {
  borderRadius?: number;
  badgeTitle: string;
  sticky?: boolean;
  color: string;
};

export const ToolTipBadge: React.FC<PropsWithChildren<Props>> = ({
  children,
  borderRadius = 2,
  badgeTitle,
  sticky = false,
  color,
}) => (
  <BaseTooltip content={children} interactive={sticky}>
    <Box
      borderTopRightRadius={borderRadius}
      borderBottomLeftRadius={borderRadius}
      bg={color}
      css={`
        position: absolute;
        padding: 0px 6px;
        top: 0px;
        right: 0px;
        font-size: 10px;
      `}
    >
      {badgeTitle}
    </Box>
  </BaseTooltip>
);
