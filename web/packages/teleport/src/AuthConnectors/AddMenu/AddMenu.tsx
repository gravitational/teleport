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
import PropTypes from 'prop-types';
import * as Icons from 'design/Icon';
import Menu, { MenuItem } from 'design/Menu';
import { ButtonPrimary } from 'design/Button';
import { AuthProviderType } from 'shared/services';

class AddMenu extends React.Component<Props> {
  static displayName = 'AddMenu';

  static propTypes = {
    onClick: PropTypes.func.isRequired,
  };

  anchorEl = null;

  state = {
    open: false,
  };

  constructor(props) {
    super(props);
    this.state = {
      open: Boolean(props.open),
    };
  }

  onOpen = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  };

  onItemClick = (kind: AuthProviderType) => {
    this.onClose();
    this.props.onClick(kind);
  };

  setRef = e => {
    this.anchorEl = e;
  };

  render() {
    const { open } = this.state;
    const { disabled = false } = this.props;
    return (
      <React.Fragment>
        <ButtonPrimary
          block
          disabled={disabled}
          setRef={this.setRef}
          onClick={this.onOpen}
        >
          NEW AUTH CONNECTOR
          <Icons.CarrotDown ml="2" fontSize="23" color="text.onDark" />
        </ButtonPrimary>
        <Menu
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          menuListCss={menuListCss}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'right',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
        >
          <MenuItem onClick={() => this.onItemClick('oidc')}>
            OIDC CONNECTOR
          </MenuItem>
          <MenuItem onClick={() => this.onItemClick('github')}>
            GITHUB CONNECTOR
          </MenuItem>
          <MenuItem onClick={() => this.onItemClick('saml')}>
            SAML CONNECTOR
          </MenuItem>
        </Menu>
      </React.Fragment>
    );
  }
}

type Props = {
  disabled?: boolean;
  onClick(kind: AuthProviderType): void;
};

const menuListCss = ({ theme }) => `
  width: 240px;
  background-color: ${theme.colors.secondary.light}

  ${MenuItem} {
    background-color: ${theme.colors.secondary.main};
    color: ${theme.colors.secondary.contrastText};
    &:hover,&:focus {
      background-color: ${theme.colors.secondary.light};
    }
  }
`;

export default AddMenu;
