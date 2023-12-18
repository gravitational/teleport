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
import Flex from 'design/Flex';

import { NavigationSwitcher } from './NavigationSwitcher';
import { NavigationCategory } from './categories';

export default {
  title: 'Teleport/Navigation',
};

const navItems = [
  { category: NavigationCategory.Management },
  { category: NavigationCategory.Resources },
];

export function SwitcherResource() {
  return (
    <NavigationSwitcher
      onChange={() => null}
      items={navItems}
      value={NavigationCategory.Resources}
    />
  );
}

export function SwitcherManagement() {
  return (
    <NavigationSwitcher
      onChange={() => null}
      items={navItems}
      value={NavigationCategory.Management}
    />
  );
}

export function SwitcherRequiresManagementAttention() {
  return (
    <Flex css={{ position: 'relative' }}>
      <NavigationSwitcher
        onChange={() => null}
        items={[
          { category: NavigationCategory.Resources },
          { category: NavigationCategory.Management, requiresAttention: true },
        ]}
        value={NavigationCategory.Resources}
      />
    </Flex>
  );
}

export function SwitcherRequiresResourcesAttention() {
  return (
    <Flex css={{ position: 'relative' }}>
      <NavigationSwitcher
        onChange={() => null}
        items={[
          { category: NavigationCategory.Resources, requiresAttention: true },
          { category: NavigationCategory.Management },
        ]}
        value={NavigationCategory.Management}
      />
    </Flex>
  );
}
