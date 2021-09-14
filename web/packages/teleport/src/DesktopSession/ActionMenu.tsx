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
import { MenuIcon, MenuItem, MenuItemIcon } from 'shared/components/MenuAction';
import * as Icons from 'design/Icon';
import { Flex } from 'design';

export default function ActionMenu(props: Props) {
  return (
    <Flex alignItems="center">
      <MenuIcon
        buttonIconProps={{
          ml: 4,
          size: 0,
          color: 'text.secondary',
          style: { fontSize: '20px' },
        }}
        menuProps={menuProps}
      >
        <MenuItem onClick={props.onDisconnect}>
          <MenuItemIcon as={Icons.PowerSwitch} mr="2" />
          Disconnect
        </MenuItem>
      </MenuIcon>
    </Flex>
  );
}

type Props = {
  onDisconnect: VoidFunction;
};

const menuListCss = () => `
  width: 250px;
`;

const menuProps = {
  menuListCss,
} as const;
