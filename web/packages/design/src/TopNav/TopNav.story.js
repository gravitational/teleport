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
import { storiesOf } from '@storybook/react';
import { withInfo } from '@storybook/addon-info';
import TopNav from './TopNav';
import TopNavItem from './TopNavItem';
import TopNavUserMenu from './TopNavUserMenu';
import MenuItem from './../Menu/MenuItem';

storiesOf('Desigin/TopNav', module)
  .addDecorator(withInfo)
  .add('TopNav component', () => {
    return (
      <TopNav height="60px">
        <TopNavItem>
          Action 1
        </TopNavItem>
        <TopNavItem>
          Action 2
        </TopNavItem>
        <MenuExample />
      </TopNav>
    );
  });

class MenuExample extends React.Component {

  state = {
    open: false,
  };

  onShow = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  onItemClick = () => {
    this.onClose();
  }

  render() {
    return (
      <TopNavUserMenu
        open={this.state.open}
        onShow={this.onShow}
        onClose={this.onClose}
        user="example@example.com" >
        <MenuItem onClick={this.onItemClick}>
          Test
        </MenuItem>
        <MenuItem onClick={this.onItemClick}>
            Test2
        </MenuItem>
      </TopNavUserMenu>
    )
  }
}