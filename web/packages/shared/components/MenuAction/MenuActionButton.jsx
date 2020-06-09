/*
Copyright 2020 Gravitational, Inc.

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
import Menu from 'design/Menu';
import { ButtonBorder } from 'design';
import { CarrotDown } from 'design/Icon';
import PropTypes from 'prop-types';

export default class MenuActionButton extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      open: props.open,
      anchorEl: null,
    };
  }

  onOpen = e => {
    e.stopPropagation();
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  render() {
    const { open } = this.state;
    const { children, menuProps, buttonProps } = this.props;
    return (
      <>
        <ButtonBorder
          height="24px"
          size="small"
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
          {...buttonProps}
        >
          OPTIONS
          <CarrotDown ml={2} mr={-2} fontSize="2" color="text.secondary" />
        </ButtonBorder>
        <Menu
          getContentAnchorEl={null}
          menuListCss={menuListCss}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
          anchorOrigin={{
            vertical: 'center',
            horizontal: 'right',
          }}
          {...menuProps}
        >
          {open && this.renderItems(children)}
        </Menu>
      </>
    );
  }

  renderItems(children) {
    const filtered = React.Children.toArray(children);
    const cloned = filtered.map(child => {
      return React.cloneElement(child, {
        onClick: this.makeOnClick(child.props.onClick),
      });
    });

    return cloned;
  }

  makeOnClick(cb) {
    return e => {
      e.stopPropagation();
      this.onClose();
      cb && cb(e);
    };
  }
}

MenuActionButton.propTypes = {
  /** displays menu */
  open: PropTypes.bool,

  /** the list of items for menu */
  children: PropTypes.node,

  /** wrap in style object to provide inline css to position button */
  buttonProps: PropTypes.object,
};

MenuActionButton.defaultProps = {
  open: false,
};

const menuListCss = () => `
  min-width: 100px;
`;
