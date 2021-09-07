/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { NavLink } from 'react-router-dom';
import { MenuIcon, MenuItem, MenuItemIcon } from 'shared/components/MenuAction';
import * as Icons from 'design/Icon';
import { Flex } from 'design';
import cfg from 'teleport/config';
import { useTheme } from 'styled-components';

export default function ActionMenu() {
  const theme = useTheme();

  return (
    <Flex alignItems="center">
      <MenuIcon
        buttonIconProps={{
          ml: 4,
          size: 0,
          style: { fontSize: '20px', color: theme.colors.text.secondary },
        }}
        menuProps={menuProps}
      >
        <MenuItem as={NavLink} to={cfg.routes.root}>
          <MenuItemIcon as={Icons.Home} mr="2" />
          Main
        </MenuItem>
      </MenuIcon>
    </Flex>
  );
}

const menuListCss = () => `
  width: 250px;
`;

const menuProps = {
  menuListCss,
  anchorOrigin: {
    vertical: 'center',
    horizontal: 'center',
  },
  transformOrigin: {
    vertical: 'top',
    horizontal: 'center',
  },
} as const;
