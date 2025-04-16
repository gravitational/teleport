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

import styled from 'styled-components';

import { Box } from 'design';

import { BadgeTitle, ToolTipNoPermBadge } from './ToolTipNoPermBadge';

export default {
  title: 'Teleport/ToolTip',
};

export const NoPermissionBadgeString = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge children={'I am a string'} />
  </SomeBox>
);

export const NoPermissionBadgeComp = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge
      children={<Box p={3}>I'm a box component with too much padding</Box>}
    />
  </SomeBox>
);

export const EnterpriseOnly = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge
      badgeTitle={BadgeTitle.LackingEnterpriseLicense}
      children={'Lacking enterprise license'}
    />
  </SomeBox>
);

export const StickyTooltip = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge
      sticky={true}
      children={
        <Box p={3}>I will only disappear after this component is unhovered</Box>
      }
    />
  </SomeBox>
);

const SomeBox = styled.div`
  width: 240px;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  position: relative;
  align-items: center;
  background-color: ${props => props.theme.colors.spotBackground[0]};
`;
