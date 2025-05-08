/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

/**
 * Used to display unified resource status info
 */
export const resourceStatusPanelWidth = 450;
/**
 * Used to display documentation/help/hint info
 */
export const generalInfoPanelWidth = 300;

export const marginTransitionCss = ({
  sidePanelOpened,
  panelWidth,
}: {
  sidePanelOpened: boolean;
  panelWidth?: number;
}) => `
  margin-right: ${sidePanelOpened ? panelWidth : '0'}px;
  transition: ${sidePanelOpened ? 'margin 150ms' : 'margin 300ms'};
`;
