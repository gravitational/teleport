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

export type Status = 'on' | 'off' | 'error' | 'warning' | 'processing';

export const ConnectionStatusIndicator = (props: {
  status: Status;
  /**
   * Color for the `on` and `processing` statuses.
   * Defaults to green (`success`).
   */
  activeStatusColor?: string;
  inline?: boolean;
  [key: string]: any;
}) => {
  const { status, inline, ...styles } = props;

  return (
    <StyledStatus
      {...styles}
      $status={status}
      $inline={inline}
      activeStatusColor={props.activeStatusColor}
    />
  );
};

const StyledStatus = styled(Box)<{
  $inline?: boolean;
  $status: Status;
  activeStatusColor?: string;
}>`
  position: relative;
  ${props => props.$inline && `display: inline-block;`}
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: 50%;

  ${props => {
    const {
      $status,
      theme,
      activeStatusColor = props.theme.colors.interactive.solid.success.default,
    } = props;

    switch ($status) {
      case 'on': {
        return { backgroundColor: activeStatusColor };
      }
      case 'processing': {
        return css`
          background-color: ${activeStatusColor};
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
          color: ${theme.colors.interactive.solid.danger.default};
          &:after {
            // This is "multiplication X" (U+2715) as "aegan check mark" (U+10102) doesn't work on
            // Windows.
            content: '✕';
            font-size: 12px;

            ${!props.$inline &&
            `position: absolute;
            top: -8px;
            `}
          }
        `;
      }
      case 'warning': {
        return css`
          color: ${theme.colors.interactive.solid.alert.default};
          &:after {
            content: '⚠';
            font-size: 12px;
            ${props.$inline &&
            `
            // This cuts out a little portion of the icon on the left. This is most clearly visible
            // on Windows. But at least it better aligns with the other statuses.
            //
            // TODO(ravicious): Rewrite this to not use weird characters to represent different
            // statuses so that all statuses properly align together.
            margin: -1px;
            `}

            ${!props.$inline &&
            `
            position: absolute;
            top: -1px;
            // Visually, -1px seems to be better aligned than -2px, especially when looking at
            // VnetWarning story.
            left: -1px;
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
