/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import * as icons from 'design/Icon';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { Menu, MenuItem } from './AdditionalActions';

export default {
  title: 'Teleterm/AdditionalActions',
};

export const MenuItems = () => {
  return (
    <MockAppContextProvider>
      <Menu
        css={`
          max-width: 300px;
        `}
      >
        <MenuItem
          closeMenu={noop}
          item={{
            Icon: icons.Code,
            isVisible: true,
            title: 'Regular item',
            onNavigate: () => {
              alert('Hello!');
            },
          }}
        />
        <MenuItem
          closeMenu={noop}
          item={{
            Icon: icons.Moon,
            isVisible: true,
            isDisabled: true,
            title: 'Disabled',
            disabledText: '…for a reason',
            onNavigate: noop,
          }}
        />
        <MenuItem
          closeMenu={noop}
          item={{
            Icon: icons.Link,
            isVisible: true,
            title: 'With a shortcut',
            onNavigate: noop,
            keyboardShortcutAction: 'newTerminalTab',
          }}
        />
        <MenuItem
          closeMenu={noop}
          item={{
            Icon: icons.List,
            isVisible: true,
            title: 'With a separator',
            prependSeparator: true,
            onNavigate: noop,
          }}
        />
        <MenuItem
          closeMenu={noop}
          item={{
            Icon: icons.Stars,
            isVisible: true,
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
