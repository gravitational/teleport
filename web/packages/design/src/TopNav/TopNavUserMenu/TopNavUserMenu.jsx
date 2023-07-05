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
import styled from 'styled-components';
import PropTypes from 'prop-types';

import { Text } from 'design';

import TopNavItem from '../TopNavItem';
import Menu from '../../Menu/Menu';

class TopNavUserMenu extends React.Component {
  static displayName = 'TopNavMenu';

  static defaultProps = {
    menuListCss: () => {},
    open: false,
  };

  static propTypes = {
    /** Callback fired when the component requests to be closed. */
    onClose: PropTypes.func,
    /** Callback fired when the component requests to be open. */
    onShow: PropTypes.func,
    /** If true the menu is visible */
    open: PropTypes.bool,
  };

  setRef = e => {
    this.btnRef = e;
  };

  render() {
    const {
      user,
      onShow,
      onClose,
      open,
      anchorOrigin,
      transformOrigin,
      children,
      menuListCss,
    } = this.props;
    const initial =
      user && user.length ? user.trim().charAt(0).toUpperCase() : '';
    const anchorEl = open ? this.btnRef : null;
    return (
      <>
        <TopNavItem
          ml="auto"
          maxWidth="250px"
          ref={this.setRef}
          onClick={onShow}
        >
          <Text fontSize="12px" bold>
            {user}
          </Text>
          <StyledAvatar>{initial}</StyledAvatar>
        </TopNavItem>
        <Menu
          menuListCss={menuListCss}
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={anchorEl}
          open={Boolean(anchorEl)}
          onClose={onClose}
        >
          {children}
        </Menu>
      </>
    );
  }
}

const StyledAvatar = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.brand.accent};
  border-radius: 50%;
  display: flex;
  font-size: 14px;
  font-weight: bold;
  justify-content: center;
  height: 32px;
  margin-left: 16px;
  width: 100%;
  max-width: 32px;
  min-width: 32px;
`;

export default TopNavUserMenu;
