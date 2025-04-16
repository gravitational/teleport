/*
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

import PropTypes from 'prop-types';
import { Component, createRef } from 'react';

import Popover from '../Popover';
import getScrollbarSize from './../utils/scrollbarSize';
import MenuList from './MenuList';

const POSITION = {
  vertical: 'top',
  horizontal: 'right',
};

class Menu extends Component {
  menuListRef = createRef();

  getContentAnchorEl = () => this.menuListRef.current?.firstChild;

  handleEntering = element => {
    const menuList = this.menuListRef.current;

    // Let's ignore that piece of logic if users are already overriding the width
    // of the menu.
    if (
      menuList &&
      element.clientHeight < menuList.clientHeight &&
      !menuList.style.width
    ) {
      const size = `${getScrollbarSize()}px`;
      menuList.style['paddingRight'] = size;
      menuList.style.width = `calc(100% + ${size})`;
    }

    if (this.props.onEntering) {
      this.props.onEntering(element);
    }
  };

  render() {
    const { children, popoverCss, menuListCss, menuListProps, ...other } =
      this.props;

    return (
      <Popover
        popoverCss={popoverCss}
        getContentAnchorEl={this.getContentAnchorEl}
        onEntering={this.handleEntering}
        anchorOrigin={POSITION}
        transformOrigin={POSITION}
        {...other}
      >
        <MenuList
          {...menuListProps}
          menuListCss={menuListCss}
          ref={this.menuListRef}
        >
          {children}
        </MenuList>
      </Popover>
    );
  }
}

Menu.propTypes = {
  /**
   * The DOM element used to set the position of the menu.
   */
  anchorEl: PropTypes.oneOfType([PropTypes.object, PropTypes.func]),
  /**
   * Menu contents, normally `MenuItem`s.
   */
  children: PropTypes.node,

  /**
   * Callback fired when the component requests to be closed.
   *
   * @param {object} event The event source of the callback
   * @param {string} reason Can be:`"escapeKeyDown"`, `"backdropClick"`, `"tabKeyDown"`
   */
  onClose: PropTypes.func,
  /**
   * Callback fired when the Menu is entering.
   */
  onEntering: PropTypes.func,
  /**
   * If `true`, the menu is visible.
   */
  open: PropTypes.bool.isRequired,
  /**
   * `popoverCss` property applied to the [`Popover`] css.
   */
  popoverCss: PropTypes.func,
  /**
   * `menuListCss` property applied to the [`MenuList`] css.
   */
  menuListCss: PropTypes.func,
  /**
   * `menuListProps` are props passed to the underlying [`MenuList`].
   */
  menuListProps: PropTypes.shape(MenuList.propTypes),
};

export default Menu;
