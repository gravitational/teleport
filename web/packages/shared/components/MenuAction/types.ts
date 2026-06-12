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

// TS: move *Positions and Origins types
// to design/Popover.js once converted to ts
type HorizontalPositions = 'left' | 'center' | 'right' | number;
type VerticalPositions = 'top' | 'center' | 'bottom' | number;
type Origins = {
  vertical: VerticalPositions;
  horizontal: HorizontalPositions;
};

export type MenuProps = {
  anchorOrigin?: Origins;
  transformOrigin?: Origins;
  // CSS supplied to MenuList to be consumed by styled-component
  menuListCss?: (props?: any) => string;
  backdropProps?: Record<string, any>;
};

export type AnchorProps = {
  // inline-styling
  style?: { [key: string]: string };
  // TS: temp handling of styled-system
  [key: string]: any;
};
