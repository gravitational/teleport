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
import { Box } from 'design';

type Status = 'on' | 'off' | 'error';

export const ConnectionStatusIndicator = (props: {
  status: Status;
  [key: string]: any;
}) => {
  const { status, ...styles } = props;
  return <StyledStatus $status={status} {...styles} />;
};

const StyledStatus = styled(Box)`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  ${(props: { $status: Status; [key: string]: any }) => {
    const { $status, theme } = props;
    let backgroundColor: string;

    switch ($status) {
      case 'on':
        backgroundColor = theme.colors.success.main;
        break;
      case 'off':
        backgroundColor = theme.colors.grey[300];
        break;
      case 'error':
        // TODO(ravicious): Don't depend on color alone, add an exclamation mark.
        backgroundColor = theme.colors.error.main;
        break;
      default:
        $status satisfies never;
    }
    return {
      backgroundColor,
    };
  }}
`;
