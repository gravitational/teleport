/**
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

import React, { PropsWithChildren } from 'react';
import Menu from 'design/Menu';
import { ButtonBorder } from 'design';
import { ChevronDown } from 'design/Icon';

import { MenuProps, AnchorProps } from './types';

type Props = MenuProps & {
  defaultOpen?: boolean;
  buttonProps?: AnchorProps;
  buttonText?: string;
  menuProps?: MenuProps;
};

export default class MenuActionIcon extends React.Component<
  PropsWithChildren<Props>
> {
  anchorEl = null;

  state = {
    open: false,
  };

  constructor(props: Props) {
    super(props);
    this.state.open = props.defaultOpen || false;
  }

  onOpen = (e: React.MouseEvent<HTMLButtonElement>) => {
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
          size="small"
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
          {...buttonProps}
        >
          {this.props.buttonText || 'Options'}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
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
            vertical: 'bottom',
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
    const cloned = filtered.map((child: React.ReactElement) => {
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

const menuListCss = () => `
  min-width: 100px;
`;
