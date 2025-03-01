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

import * as icons from 'design/Icon';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { Menu, MenuListItem } from './Menu';

export default {
  title: 'Teleterm/Menu',
};

export const MenuItems = () => {
  return (
    <MockAppContextProvider>
      <Menu
        css={`
          max-width: 300px;
        `}
      >
        <MenuListItem
          closeMenu={noop}
          item={{
            Icon: icons.Code,
            title: 'Regular item',
            onNavigate: () => {
              alert('Hello!');
            },
          }}
        />
        <MenuListItem
          closeMenu={noop}
          item={{
            Icon: icons.Moon,
            isDisabled: true,
            title: 'Disabled',
            disabledText: '…for a reason',
            onNavigate: noop,
          }}
        />
        <MenuListItem
          closeMenu={noop}
          item={{
            Icon: icons.Link,
            title: 'With a shortcut',
            onNavigate: noop,
            keyboardShortcutAction: 'newTerminalTab',
          }}
        />
        <MenuListItem
          closeMenu={noop}
          item={{
            Icon: icons.ListThin,
            title: 'With a separator',
            prependSeparator: true,
            onNavigate: noop,
          }}
        />
        <MenuListItem
          closeMenu={noop}
          item={{
            Icon: icons.Stars,
            title: 'With everything',
            isDisabled: true,
            disabledText: 'Lorem ipsum dolor sit amet',
            prependSeparator: true,
            onNavigate: noop,
            keyboardShortcutAction: 'previousTab',
          }}
        />
      </Menu>
    </MockAppContextProvider>
  );
};

const noop = () => {};
