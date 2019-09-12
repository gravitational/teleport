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
import ReactDOM from 'react-dom';
import getScrollbarSize from 'dom-helpers/util/scrollbarSize';
import Popover from '../Popover';
import MenuList from './MenuList';

const POSITION = {
  vertical: 'top',
  horizontal: 'right',
};

class Menu extends React.Component {

  getContentAnchorEl = () => {
    if (this.menuListRef.selectedItemRef) {
      return ReactDOM.findDOMNode(this.menuListRef.selectedItemRef);
    }

    return ReactDOM.findDOMNode(this.menuListRef).firstChild;
  };

  handleMenuListRef = ref => {
    this.menuListRef = ref;
  };

  handleEntering = element => {
    const menuList = ReactDOM.findDOMNode(this.menuListRef);

    // Let's ignore that piece of logic if users are already overriding the width
    // of the menu.
    if (menuList && element.clientHeight < menuList.clientHeight && !menuList.style.width) {
      const size = `${getScrollbarSize()}px`;
      menuList.style['paddingRight'] = size;
      menuList.style.width = `calc(100% + ${size})`;
    }

    if (this.props.onEntering) {
      this.props.onEntering(element);
    }
  };

  render() {
    const {
      children,
      popoverCss,
      menuListCss,
      ...other
    } = this.props;


    return (
      <Popover
        popoverCss={popoverCss}
        getContentAnchorEl={this.getContentAnchorEl}
        onEntering={this.handleEntering}
        anchorOrigin={POSITION}
        transformOrigin={POSITION}
        {...other}
      >
        <MenuList menuListCss={menuListCss} ref={this.handleMenuListRef}>
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
};

export default Menu;
