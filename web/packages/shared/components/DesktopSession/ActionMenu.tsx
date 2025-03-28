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
import * as Icons from 'design/Icon';
import { MenuIcon, MenuItem, MenuItemIcon } from 'shared/components/MenuAction';

export default function ActionMenu(props: Props) {
  const { showShareDirectory, onShareDirectory, onDisconnect, onCtrlAltDel } =
    props;

  return (
    <Flex alignItems="center">
      <MenuIcon
        buttonIconProps={{
          ml: 4,
          size: 0,
          color: 'text.slightlyMuted',
          style: { fontSize: '20px' },
        }}
        menuProps={menuProps}
      >
        {showShareDirectory && (
          <MenuItem onClick={onShareDirectory}>
            <MenuItemIcon as={Icons.FolderPlus} mr="2" />
            Share Directory
          </MenuItem>
        )}
        <MenuItem onClick={onCtrlAltDel}>
          <MenuItemIcon as={Icons.Keyboard} mr="2" />
          Send Ctrl+Alt+Del
        </MenuItem>
        <MenuItem onClick={onDisconnect}>
          <MenuItemIcon as={Icons.PowerSwitch} mr="2" />
          Disconnect
        </MenuItem>
      </MenuIcon>
    </Flex>
  );
}

type Props = {
  showShareDirectory: boolean;
  onShareDirectory: VoidFunction;
  onDisconnect: VoidFunction;
  onCtrlAltDel: VoidFunction;
};

const menuListCss = () => `
  width: 250px;
`;

const menuProps = {
  menuListCss,
} as const;
