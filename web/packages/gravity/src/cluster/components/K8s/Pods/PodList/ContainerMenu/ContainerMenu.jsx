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
import ReactDOM from 'react-dom';
import PropTypes from 'prop-types';
import styled from 'styled-components';
import { Flex, Text, ButtonOutlined, ButtonPrimary } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import * as Icons from 'design/Icon';
import cfg from 'gravity/config';
import { NavLink } from 'gravity/components/Router';

class ContainerMenu extends React.Component {
  static displayName = 'ContainerMenu';

  menuRef = React.createRef();

  constructor(props) {
    super(props);
    this.state = {
      open: false,
      anchorEl: null,
    };
  }

  onOpen = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  setButtonRef = e => {
    this.anchorEl = e;
  };

  // get the first menu item to align it with the container button
  getContentAnchorEl = () => {
    return ReactDOM.findDOMNode(this.menuRef.current).children[1];
  };

  render() {
    const { logsEnabled, container, anchorOrigin, transformOrigin, ...styles } = this.props;
    const { open } = this.state;
    const { logUrl, name, sshLogins, pod, serverId, namespace } = container;

    return (
      <React.Fragment>
        <ButtonOutlined
          size="small"
          p="1"
          setRef={this.setButtonRef}
          onClick={this.onOpen}
          {...styles}
        >
          {name}
          <Icons.CarrotDown ml="2" fontSize="3" color="text.onDark" />
        </ButtonOutlined>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          getContentAnchorEl={this.getContentAnchorEl}
        >
          <LoginItemList
            logsEnabled={logsEnabled}
            ref={this.menuRef}
            serverId={serverId}
            logUrl={logUrl}
            title={name}
            namespace={namespace}
            pod={pod}
            onClick={this.onClose}
            container={name}
            logins={sshLogins}
          />
        </Menu>
      </React.Fragment>
    );
  }
}

const LoginItemList = React.forwardRef(
  (
    { onClick, logins, title, logsEnabled, serverId, logUrl, container, pod, namespace },
    ref
  ) => {
    logins = logins || [];
    const $menuItems = logins.map((login, key) => {
      const url = cfg.getConsoleInitPodSessionRoute({ login, serverId, container, pod, namespace });
      return (
        <MenuItem
          px="2"
          mx="2"
          as={SyledMenuItem}
          href={url}
          key={key}
          target="_blank"
          onClick={onClick}
        >
          {login}
        </MenuItem>
      );
    });

    return (
      <Flex ref={ref} flexDirection="column" minWidth="200px" pb="2">
        <Text px="2" fontSize="11px" mb="2" color="grey.400" bg="subtle">
          SSH - {title}
        </Text>
        {$menuItems}
        {logsEnabled && (
          <ButtonPrimary mt="3" mb="2" mx="3" size="small" as={NavLink} to={logUrl}>
            View Logs
          </ButtonPrimary>
        )}
      </Flex>
    );
  }
);

LoginItemList.propTypes = {
  logsEnabled: PropTypes.bool.isRequired,
}


const SyledMenuItem = styled.a`
  color: ${props => props.theme.colors.grey[400]};
  font-size: 12px;
  border-bottom: 1px solid ${props => props.theme.colors.subtle};
  min-height: 32px;
  &:hover {
    color: ${props => props.theme.colors.link};
  }

  :last-child {
    border-bottom: none;
  }
`;

export default ContainerMenu;
