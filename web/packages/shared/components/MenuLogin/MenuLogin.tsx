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
import { NavLink } from 'react-router-dom';
import Menu, { MenuItem } from 'design/Menu';
import { space } from 'design/system';
import { MenuLoginProps } from './types';
import { ButtonBorder, Flex } from 'design';
import { CarrotDown } from 'design/Icon';

export class MenuLogin extends React.Component<MenuLoginProps> {
  static displayName = 'MenuLogin';

  anchorEl = React.createRef();

  state = {
    logins: [],
    open: false,
    anchorEl: null,
  };

  onOpen = () => {
    const logins = this.props.getLoginItems();
    this.setState({
      logins,
      open: true,
    });
  };

  onItemClick = (e: React.MouseEvent<HTMLAnchorElement>, login: string) => {
    this.onClose();
    this.props.onSelect(e, login);
  };

  onClose = () => {
    this.setState({ open: false });
  };

  onKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && e.currentTarget.value) {
      this.onClose();
      this.props.onSelect(e, e.currentTarget.value);
    }
  };

  render() {
    const { anchorOrigin, transformOrigin } = this.props;
    const placeholder = this.props.placeholder || 'Enter login name…';
    const { open, logins } = this.state;
    return (
      <React.Fragment>
        <ButtonBorder
          height="24px"
          size="small"
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
        >
          CONNECT
          <CarrotDown ml={2} mr={-2} fontSize="2" color="text.secondary" />
        </ButtonBorder>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          getContentAnchorEl={null}
        >
          <LoginItemList
            logins={logins}
            onKeyPress={this.onKeyPress}
            onClick={this.onItemClick}
            placeholder={placeholder}
          />
        </Menu>
      </React.Fragment>
    );
  }
}

export const LoginItemList = ({ logins, onClick, onKeyPress, placeholder }) => {
  logins = logins || [];
  const $menuItems = logins.map((item, key) => {
    const { login, url } = item;
    return (
      <StyledMenuItem
        key={key}
        px="2"
        mx="2"
        as={url ? NavLink : StyledButton}
        to={url}
        onClick={(e: Event) => {
          onClick(e, login);
        }}
      >
        {login}
      </StyledMenuItem>
    );
  });

  return (
    <Flex flexDirection="column">
      <Input
        p="2"
        m="2"
        onKeyPress={onKeyPress}
        type="text"
        autoFocus
        placeholder={placeholder}
        autoComplete="off"
      />
      {$menuItems}
    </Flex>
  );
};

const StyledButton = styled.button`
  color: inherit;
  border: none;
  flex: 1;
`;

const StyledMenuItem = styled(MenuItem)(
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

const Input = styled.input(
  ({ theme }) => `
  background: ${theme.colors.subtle};
  border: 1px solid ${theme.colors.subtle};
  border-radius: 4px;
  box-sizing: border-box;
  color: ${theme.colors.grey[900]};
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
