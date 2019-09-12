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
import cfg from 'teleport/config';

class MenuLogin extends React.Component {
  static displayName = 'MenuLogin';

  constructor(props) {
    super(props);
    this.state = {
      open: false,
      anchorEl: null,
    };
  }

  openTerminal(login) {
    const { serverId } = this.props;
    const url = cfg.getConsoleConnectRoute({ login, serverId });
    window.open(url); //, "", "toolbar=yes,scrollbars=yes,resizable=yes");
  }

  onOpen = () => {
    this.setState({ open: true });
  };

  onItemClick = () => {
    this.onClose();
  };

  onClose = () => {
    this.setState({ open: false });
  };

  onKeyPress = e => {
    if (e.key === 'Enter' && e.target.value) {
      this.onClose();
      this.openTerminal(e.target.value);
    }
  };

  render() {
    const {
      logins,
      serverId,
      clusterId,
      anchorOrigin,
      transformOrigin,
    } = this.props;

    const { open } = this.state;

    return (
      <React.Fragment>
        <StyledSessionButton
          px="2"
          ref={e => (this.anchorEl = e)}
          onClick={this.onOpen}
        >
          <Icons.Cli as={StyledCliIcon} />
          <Icons.CarrotDown as={StyledCarrotIcon} />
        </StyledSessionButton>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
        >
          <LoginItemList
            clusterId={clusterId}
            logins={logins}
            serverId={serverId}
            onKeyPress={this.onKeyPress}
            onClick={this.onItemClick}
          />
        </Menu>
      </React.Fragment>
    );
  }
}

export const LoginItemList = ({
  serverId,
  clusterId,
  logins,
  onClick,
  onKeyPress,
}) => {
  logins = logins || [];
  const $menuItems = logins.map((login, key) => {
    const url = cfg.getConsoleConnectRoute({ clusterId, login, serverId });
    return (
      <MenuItem
        key={key}
        px="2"
        mx="2"
        as={StyledMenuItem}
        href={url}
        target="_blank"
        onClick={() => onClick(login)}
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

const StyledMenuItem = styled.a`
  color: ${props => props.theme.colors.grey[400]};
  font-size: 12px;
  border-bottom: 1px solid ${props => props.theme.colors.subtle};
  min-height: 32px;
  &:hover {
    color: ${props => props.theme.colors.link};
  }

  :last-child {
    border-bottom: none;
    margin-bottom: 8px;
  }
`;

const StyledCliIcon = styled.div`
  opacity: 0.87;
`;

const StyledCarrotIcon = styled.div`
  opacity: 0.24;
`;

const StyledSessionButton = styled.button`
  display: flex;
  justify-content: space-between;
  outline-style: none;
  outline-width: 0px;
  -webkit-appearance: none;
  -webkit-tap-highlight-color: transparent;
  align-items: center;
  background: ${props => props.theme.colors.bgTerminal};
  border: 1px solid ${props => props.theme.colors.bgTerminal};
  border-radius: 2px;
  box-sizing: border-box;
  box-shadow: 0 0 2px rgba(0, 0, 0, 0.12), 0 2px 2px rgba(0, 0, 0, 0.24);
  color: ${props => props.theme.colors.primary};
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
    border: 1px solid ${props => props.theme.colors.success};
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
    ${StyledCliIcon} {
      opacity: 1;
    }
    ${StyledCarrotIcon} {
      opacity: 0.56;
    }
  }

  ${space}
`;

const Input = styled.input`
  background: ${props => props.theme.colors.subtle};
  border: 1px solid ${props => props.theme.colors.subtle};
  border-radius: 4px;
  box-sizing: border-box;
  color: #263238;
  height: 32px;
  outline: none;

  &:focus {
    background: ${props => props.theme.colors.light};
    border 1px solid ${props => props.theme.colors.link};
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, .24);
  }

  ::placeholder {
    color: ${props => props.theme.colors.grey[100]};
  }

  ${space}
`;

export default MenuLogin;
