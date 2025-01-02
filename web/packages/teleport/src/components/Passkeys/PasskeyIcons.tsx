/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';

import * as Icon from 'design/Icon';

export function PasskeyIcons() {
  return (
    <>
      <OverlappingChip>
        <Icon.FingerprintSimple p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.UsbDrive p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.UserFocus p={2} />
      </OverlappingChip>
      <OverlappingChip>
        <Icon.DeviceMobileCamera p={2} />
      </OverlappingChip>
    </>
  );
}

const OverlappingChip = styled.span`
  display: inline-block;
  background: ${props => props.theme.colors.levels.surface};
  border: ${props => props.theme.borders[1]};
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 50%;
  margin-right: -6px;
`;
