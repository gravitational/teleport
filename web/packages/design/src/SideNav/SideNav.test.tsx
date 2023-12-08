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
import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import * as Icons from '../Icon';

import SideNav, { SideNavItem, SideNavItemIcon } from './index';

test('renders: SideNav, SideNavItem, SideNavItemIcon', () => {
  render(
    <SideNav data-testid="parent">
      <SideNavItem data-testid="item">
        <SideNavItemIcon data-testid="icon" as={Icons.Apple} />
        Item 1
      </SideNavItem>
    </SideNav>
  );

  expect(screen.getByTestId('parent')).toBeInTheDocument();
  expect(screen.getByTestId('item')).toBeInTheDocument();
  expect(screen.getByTestId('icon')).toBeInTheDocument();
});
