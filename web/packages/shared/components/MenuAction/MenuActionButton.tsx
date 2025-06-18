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

import React, { PropsWithChildren, Ref } from 'react';
import styled from 'styled-components';

import { Button, ButtonBorder } from 'design';
import { ChevronDown } from 'design/Icon';
import Menu from 'design/Menu';

import { AnchorProps, MenuProps } from './types';

type Props = MenuProps & {
  defaultOpen?: boolean;
  buttonProps?: AnchorProps;
  buttonText?: React.ReactNode;
  menuProps?: MenuProps;

  // If present, button text is not used, and a square icon button is rendered instead of a border button
  icon?: React.ReactNode;
};

export default function MenuActionIcon({
  ref,
  ...otherProps
}: PropsWithChildren<Props> & { ref?: Ref<HTMLButtonElement> }) {
  // Since React class components can't forward refs, we wrap it in a function component.
  // This lets HoverTooltip access the ref to attach the tooltip to it.
  return <InnerMenuActionIcon {...otherProps} forwardedRef={ref} />;
}

class InnerMenuActionIcon extends React.Component<
  PropsWithChildren<Props & { forwardedRef?: Ref<HTMLButtonElement> }>
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

  private assignRef(e: HTMLButtonElement) {
    this.anchorEl = e;
    const { forwardedRef } = this.props;
    if (typeof forwardedRef === 'function') {
      forwardedRef(e);
    } else if (forwardedRef && typeof forwardedRef === 'object') {
      forwardedRef.current = e;
    }
  }

  render() {
    const { open } = this.state;
    const { children, menuProps, buttonProps, icon } = this.props;
    return (
      <>
        {icon ? (
          <FilledButtonIcon
            intent="neutral"
            ref={e => {
              this.assignRef(e);
            }}
            onClick={this.onOpen}
            {...buttonProps}
          >
            {icon}
          </FilledButtonIcon>
        ) : (
          <ButtonBorder
            size="small"
            ref={e => {
              this.assignRef(e);
            }}
            onClick={this.onOpen}
            {...buttonProps}
          >
            {this.props.buttonText || 'Options'}
            <ChevronDown
              ml={2}
              size="small"
              color={buttonProps?.color || 'text.slightlyMuted'}
            />
          </ButtonBorder>
        )}
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
    const cloned = filtered.map((child: React.ReactElement<any>) => {
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

const FilledButtonIcon = styled(Button)`
  width: 32px;
  height: 32px;
  padding: 0;
`;
