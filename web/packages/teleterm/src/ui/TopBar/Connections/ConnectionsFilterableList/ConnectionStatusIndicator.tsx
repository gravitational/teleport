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

import styled, { css } from 'styled-components';

import { blink, Box } from 'design';

type Status = 'on' | 'off' | 'error' | 'warning' | 'processing';

export const ConnectionStatusIndicator = (props: {
  status: Status;
  inline?: boolean;
  [key: string]: any;
}) => {
  const { status, inline, ...styles } = props;

  return <StyledStatus {...styles} $status={status} $inline={inline} />;
};

const StyledStatus = styled(Box)<{ $inline?: boolean; $status: Status }>`
  position: relative;
  ${props => props.$inline && `display: inline-block;`}
  width: 8px;
  height: 8px;
  border-radius: 50%;

  ${props => {
    const { $status, theme } = props;

    switch ($status) {
      case 'on': {
        return { backgroundColor: theme.colors.success.main };
      }
      case 'processing': {
        return css`
          background-color: ${props => props.theme.colors.success.main};
          animation: ${blink} 1.4s ease-in-out;
          animation-iteration-count: infinite;
        `;
      }
      case 'off': {
        return { border: `1px solid ${theme.colors.grey[300]}` };
      }
      case 'error': {
        // Using text instead of an icon because any icon used here would be smaller than the
        // rounded divs used to represent on and off states.
        //
        // A red circle was not used to avoid differentiating states only by color.
        //
        // The spacing has been painstakingly adjusted so that the cross is rendered pretty much at
        // the same spot as a rounded div would have been.
        //
        // To verify that the position of the cross is correct, move the &:after pseudoselector
        // outside of this switch to StyledStatus.
        return css`
          color: ${theme.colors.error.main};
          &:after {
            content: '𐄂';
            font-size: 19px;

            ${!props.$inline &&
            `position: absolute;
            top: -3px;
            left: -1px;
            line-height: 8px;`}
          }
        `;
      }
      case 'warning': {
        return css`
          color: ${theme.colors.warning.main};
          &:after {
            content: '⚠';
            font-size: 12px;

            ${!props.$inline &&
            `
            position: absolute;
            top: -1px;
            left: -2px;
            line-height: 8px;
            `}
          }
        `;
      }
      default: {
        $status satisfies never;
      }
    }
  }}
`;
