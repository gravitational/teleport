/*
Copyright 2019 Gravitational, Inc.

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

import * as Icons from '../Icon';

import SideNav, { SideNavItem, SideNavItemIcon } from './index';

export default {
  title: 'Design/SideNav',
};

export const Sample = () => (
  <SideNav static>
    <SideNavItem>Item 1</SideNavItem>
    <SideNavItem>Item 2</SideNavItem>
    <SideNavItem>Item 3</SideNavItem>
  </SideNav>
);

export const SampleWithIcons = () => (
  <SideNav static>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Apple} />
      Item 1
    </SideNavItem>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Cash} />
      Item 2
    </SideNavItem>
    <SideNavItem>
      <SideNavItemIcon as={Icons.Windows} />
      Item 3
    </SideNavItem>
  </SideNav>
);
