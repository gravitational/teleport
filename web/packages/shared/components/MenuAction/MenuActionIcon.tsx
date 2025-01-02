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

import { ButtonIcon } from 'design';
import { MoreHoriz } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import Menu from 'design/Menu';

import { AnchorProps, MenuProps } from './types';

export default class MenuActionIcon extends React.Component<
  PropsWithChildren<Props>
> {
  static defaultProps = {
    Icon: MoreHoriz,
  };
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
    const { children, buttonIconProps, menuProps, Icon } = this.props;
    return (
      <>
        <ButtonIcon
          {...buttonIconProps}
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
          data-testid="button"
        >
          <Icon size="medium" />
        </ButtonIcon>
        <Menu
          getContentAnchorEl={null}
          menuListCss={menuListCss}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
          {...menuProps}
        >
          {open && this.renderItems(children)}
        </Menu>
      </>
    );
  }

  renderItems(children: React.ReactNode) {
    const filtered = React.Children.toArray(children) as React.ReactElement[];
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

const menuListCss = () => `
  min-width: 100px;
`;

type Props = MenuProps & {
  defaultOpen?: boolean;
  buttonIconProps?: AnchorProps;
  menuProps?: MenuProps;
  Icon?: React.ComponentType<IconProps>;
};
