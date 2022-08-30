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

import { ButtonPrimary } from '../Button';
import Box from '../Box';
import Flex from '../Flex';
import * as Icons from '../Icon';

import MenuItemIcon from './MenuItemIcon';
import MenuItem from './MenuItem';
import Menu from './Menu';

export default {
  title: 'Design/Menu',
};

export const PlacementExample = () => (
  <Flex justifyContent="space-between">
    <SimpleMenu text="Menu to right">
      <MenuItem>Test</MenuItem>
      <MenuItem>Test2</MenuItem>
    </SimpleMenu>
    <SimpleMenu
      text="Menu in center"
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'center',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'center',
      }}
    >
      <MenuItem>Test</MenuItem>
      <MenuItem>Test2</MenuItem>
      <MenuItem>
        <ButtonPrimary mt={2} mb={2} block>
          Logout
        </ButtonPrimary>
      </MenuItem>
    </SimpleMenu>
    <SimpleMenu
      text="Menu to left"
      anchorOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
    >
      <MenuItem>Test</MenuItem>
      <MenuItem>Test2</MenuItem>
    </SimpleMenu>
  </Flex>
);

export const IconExample = () => (
  <Menu
    anchorOrigin={{
      vertical: 'bottom',
      horizontal: 'center',
    }}
    transformOrigin={{
      vertical: 'top',
      horizontal: 'center',
    }}
    open={true}
  >
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.Apple} />
      Test
    </MenuItem>
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.Cash} />
      Test
    </MenuItem>
    <MenuItem data-testid="item">
      <MenuItemIcon data-testid="icon" as={Icons.CircleArrowLeft} />
      Test
    </MenuItem>
  </Menu>
);

class SimpleMenu extends React.Component {
  state = {
    anchorEl: null,
  };

  handleClickListItem = event => {
    this.setState({ anchorEl: event.currentTarget });
  };

  handleMenuItemClick = () => {
    this.setState({ anchorEl: null });
  };

  handleClose = () => {
    this.setState({ anchorEl: null });
  };

  render() {
    const { text, anchorOrigin, transformOrigin, children } = this.props;
    const { anchorEl } = this.state;
    return (
      <Box m={11} textAlign="center">
        <ButtonPrimary size="small" onClick={this.handleClickListItem}>
          {text}
        </ButtonPrimary>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={anchorEl}
          open={Boolean(anchorEl)}
          onClose={this.handleClose}
        >
          {children}
        </Menu>
      </Box>
    );
  }
}
