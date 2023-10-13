/**
 * Copyright 2023 Gravitational, Inc.
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
