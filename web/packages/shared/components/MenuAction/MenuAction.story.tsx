/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Flex } from 'design';

import { Cog } from 'design/Icon';

import { MenuIcon, MenuButton, MenuItem } from '.';

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
