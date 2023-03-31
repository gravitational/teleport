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
import 'react-day-picker/lib/style.css';
import { ButtonOutlined } from 'design';
import { CarrotDown } from 'design/Icon';
import Menu from 'design/Menu';
import defaultTheme from 'design/theme';

type Props = {
  title: string;
  disabled: boolean;
  children: React.ReactElement[];
  [index: string]: any;
};

type State = {
  open: boolean;
  anchorEl: HTMLElement;
};

export default class ButtonOptions extends React.Component<Props, State> {
  anchorEl: HTMLElement;

  constructor(props: Props) {
    super(props);
    this.setState({
      open: props.open,
    });
  }

  onOpen = e => {
    e.stopPropagation();
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  render() {
    const props = this.props;

    return (
      <>
        <StyledButton
          size="small"
          width="180px"
          disabled={props.disabled}
          ml={props.ml}
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
        >
          {props.title}
          <CarrotDown
            ml={2}
            style={{ position: 'absolute', top: '6px', right: '16px' }}
            fontSize="3"
            color="levels.elevated"
          />
        </StyledButton>
        <Menu
          menuListCss={menuListCss}
          anchorEl={this.state.anchorEl}
          open={open}
          onClose={this.onClose}
          anchorOrigin={{
            vertical: 'center',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
        >
          {open && this.renderItems(props.children)}
        </Menu>
      </>
    );
  }

  renderItems(children: React.ReactElement[]) {
    const filtered = React.Children.toArray(children);
    const cloned = filtered.map((child: any) => {
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

const StyledButton = styled(ButtonOutlined)`
  border-color: ${props => props.theme.colors.levels.elevated};
  height: 32px;
  padding: 0 40px 0 24px;
  width: auto;
`;

StyledButton.defaultProps = {
  theme: defaultTheme,
};
