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
import PropTypes from 'prop-types';
import { Image, Text } from 'design';
import TopNavItem from '../TopNavItem';
import Menu from '../../Menu/Menu';
import defaultAvatar from './avatar.png';

class TopNavUserMenu extends React.Component {
  static displayName = 'TopNavMenu';

  static defaultProps = {
    menuListCss: () => {},
    avatar: defaultAvatar,
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
      avatar,
      anchorOrigin,
      transformOrigin,
      children,
      menuListCss,
    } = this.props;

    const anchorEl = open ? this.btnRef : null;
    return (
      <React.Fragment>
        <TopNavItem
          ml="auto"
          maxWidth="250px"
          ref={this.setRef}
          onClick={onShow}
        >
          <Text typography="subtitle2" bold>
            {user}
          </Text>
          <Image height="24px" ml="3" mr="2" src={avatar} />
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
      </React.Fragment>
    );
  }
}

export default TopNavUserMenu;
