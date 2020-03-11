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
import Menu, { MenuItem } from 'design/Menu';
import { space } from 'design/system';
import * as Icons from 'design/Icon';
import { MenuSshLoginProps } from './types';

class MenuSshLogin extends React.Component<MenuSshLoginProps> {
  static displayName = 'MenuSshLogin';

  anchorEl = React.createRef();

  state = {
    logins: [],
    open: false,
    anchorEl: null,
  };

  onOpen = () => {
    const logins = this.props.onOpen();
    this.setState({
      logins,
      open: true,
    });
  };

  onItemClick = login => {
    this.onClose();
    this.props.onSelect(login);
  };

  onClose = () => {
    this.setState({ open: false });
  };

  onKeyPress = e => {
    if (e.key === 'Enter' && e.target.value) {
      this.onClose();
      this.props.onSelect(e.target.value);
    }
  };

  render() {
    const { anchorOrigin, transformOrigin } = this.props;
    const { open, logins } = this.state;
    return (
      <React.Fragment>
        <StyledSessionButton px="2" ref={this.anchorEl} onClick={this.onOpen}>
          <Icons.Cli as={StyledCliIcon} />
          <Icons.CarrotDown as={StyledCarrotIcon} />
        </StyledSessionButton>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl.current}
          open={open}
          onClose={this.onClose}
        >
          <LoginItemList
            logins={logins}
            onKeyPress={this.onKeyPress}
            onClick={this.onItemClick}
          />
        </Menu>
      </React.Fragment>
    );
  }
}

export const LoginItemList = ({ logins, onClick, onKeyPress }) => {
  logins = logins || [];
  const $menuItems = logins.map((item, key) => {
    const { login, url } = item;
    return (
      <MenuItem
        key={key}
        px="2"
        mx="2"
        as={StyledMenuItem}
        href={url}
        onClick={e => {
          e.preventDefault();
          onClick(login);
        }}
      >
        {login}
      </MenuItem>
    );
  });

  return (
    <React.Fragment>
      <Input
        p="2"
        mx="2"
        my="2"
        onKeyPress={onKeyPress}
        type="text"
        autoFocus
        placeholder="Enter login name..."
      />
      {$menuItems}
    </React.Fragment>
  );
};

const StyledMenuItem = styled.a(
  ({ theme }) => `
  color: ${theme.colors.grey[400]};
  font-size: 12px;
  border-bottom: 1px solid ${theme.colors.subtle};
  min-height: 32px;
  &:hover {
    color: ${theme.colors.link};
  }

  :last-child {
    border-bottom: none;
    margin-bottom: 8px;
  }
`
);

const StyledCliIcon = styled.div`
  opacity: 0.87;
`;

const StyledCarrotIcon = styled.div`
  opacity: 0.24;
`;

const StyledSessionButton = styled.button(
  ({ theme }) => `
  display: flex;
  justify-content: space-between;
  outline-style: none;
  outline-width: 0px;
  -webkit-appearance: none;
  -webkit-tap-highlight-color: transparent;
  align-items: center;
  background: ${theme.colors.bgTerminal};
  border: 1px solid ${theme.colors.bgTerminal};
  border-radius: 2px;
  box-sizing: border-box;
  box-shadow: 0 0 2px rgba(0, 0, 0, 0.12), 0 2px 2px rgba(0, 0, 0, 0.24);
  color: ${theme.colors.primary};
  cursor: pointer;
  height: 24px;
  width: 56px;

  transition: all 0.3s;
  > * {
    transition: all 0.3s;
  }

  :focus {
    outline: none;
  }

  ::-moz-focus-inner {
    border: 0;
  }

  :hover,
  :focus {
    border: 1px solid ${theme.colors.success};
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
    ${StyledCliIcon} {
      opacity: 1;
    }
    ${StyledCarrotIcon} {
      opacity: 0.56;
    }
  }
`,
  space
);

const Input = styled.input(
  ({ theme }) => `
  background: ${theme.colors.subtle};
  border: 1px solid ${theme.colors.subtle};
  border-radius: 4px;
  box-sizing: border-box;
  color: #263238;
  height: 32px;
  outline: none;

  &:focus {
    background: ${theme.colors.light};
    border 1px solid ${theme.colors.link};
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, .24);
  }

  ::placeholder {
    color: ${theme.colors.grey[100]};
  }
`,
  space
);

export default MenuSshLogin;
