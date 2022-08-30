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

import TopNav from './TopNav';
import TopNavItem from './TopNavItem';
import TopNavUserMenu from './TopNavUserMenu';
import MenuItem from './../Menu/MenuItem';

export default {
  title: 'Design/TopNav',
};

export const Sample = () => {
  const [visible, setVisible] = React.useState(false);

  function onShow() {
    setVisible(true);
  }

  function onClose() {
    setVisible(false);
  }

  function onItemClick() {
    onClose();
  }

  return (
    <TopNav height="60px">
      <TopNavItem>MenuItem1</TopNavItem>
      <TopNavItem>MenuItem2</TopNavItem>
      <TopNavUserMenu
        open={visible}
        onShow={onShow}
        onClose={onClose}
        user="example@example.com"
      >
        <MenuItem onClick={onItemClick}>Test</MenuItem>
        <MenuItem onClick={onItemClick}>Test2</MenuItem>
      </TopNavUserMenu>
    </TopNav>
  );
};
