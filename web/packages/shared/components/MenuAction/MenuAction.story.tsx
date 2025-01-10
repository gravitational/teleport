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

import { Flex } from 'design';
import { Cog } from 'design/Icon';

import { MenuButton, MenuIcon, MenuItem } from '.';

export default {
  title: 'Shared/MenuAction',
};

export const Menu = () => (
  <Flex gap={11} flexWrap="wrap">
    <Flex flexDirection="column">
      MenuIcon
      <MenuIcon
        menuProps={{
          anchorOrigin: { vertical: 'top', horizontal: 'left' },
          transformOrigin: { vertical: 'top', horizontal: 'left' },
        }}
      >
        <MenuItem>Edit…</MenuItem>
        <MenuItem>Delete…</MenuItem>
      </MenuIcon>
    </Flex>

    <Flex flexDirection="column">
      MenuIcon with a custom icon
      <MenuIcon
        Icon={Cog}
        menuProps={{
          anchorOrigin: { vertical: 'top', horizontal: 'left' },
          transformOrigin: { vertical: 'top', horizontal: 'left' },
        }}
      >
        <MenuItem>Edit…</MenuItem>
        <MenuItem>Delete…</MenuItem>
      </MenuIcon>
    </Flex>

    <Flex flexDirection="column">
      MenuButton
      <MenuButton>
        <MenuItem>Edit…</MenuItem>
        <MenuItem>Delete…</MenuItem>
      </MenuButton>
    </Flex>
  </Flex>
);
