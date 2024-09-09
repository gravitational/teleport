/*
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

import * as Icons from '../Icon';

import SideNav, { SideNavItem, SideNavItemIcon } from './index';

export default {
  title: 'Design/SideNav',
};

export const Sample = () => (
  <SideNav static>
    <SideNavItem>Item 1</SideNavItem>
    <SideNavItem>Item 2</SideNavItem>
    <SideNavItem>Item 3</SideNavItem>
  </SideNav>
);

export const SampleWithIcons = () => (
  <SideNav static>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Apple} />
      Item 1
    </SideNavItem>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Cash} />
      Item 2
    </SideNavItem>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Windows} />
      Item 3
    </SideNavItem>
  </SideNav>
);
