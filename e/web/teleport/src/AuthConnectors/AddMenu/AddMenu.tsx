import { Component, Fragment } from 'react';
import PropTypes from 'prop-types';
import * as Icons from 'design/Icon';
import Menu, { MenuItem } from 'design/Menu';
import { Button } from 'design/Button';
import { AuthProviderType } from 'shared/services';

class AddMenu extends Component<Props> {
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
    const {
      disabled = false,
      isOidcLocked = false,
      isSamlLocked = false,
    } = this.props;
    return (
      <Fragment>
        <Button
          intent="primary"
          fill="border"
          block
          disabled={disabled}
          setRef={this.setRef}
          onClick={this.onOpen}
        >
          New Auth Connector
          <Icons.ChevronDown ml={2} size="small" />
        </Button>
        <Menu
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          menuListCss={menuListCss}
          popoverCss={() => `margin-top: 36px;`}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'right',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
        >
          {!isOidcLocked && (
            <MenuItem onClick={() => this.onItemClick('oidc')}>
              OIDC Connector
            </MenuItem>
          )}
          <MenuItem onClick={() => this.onItemClick('github')}>
            GitHub Connector
          </MenuItem>
          {!isSamlLocked && (
            <MenuItem onClick={() => this.onItemClick('saml')}>
              SAML Connector
            </MenuItem>
          )}
        </Menu>
      </Fragment>
    );
  }
}

type Props = {
  disabled?: boolean;
  onClick(kind: AuthProviderType): void;
  isOidcLocked: boolean;
  isSamlLocked: boolean;
};

const menuListCss = ({ theme }) => `
  width: 240px;
  background-color: ${theme.colors.brand}

  ${MenuItem} {
    background-color: ${theme.colors.brand};
    color: ${theme.colors.text.main};
    &:hover,&:focus {
      background-color: ${theme.colors.brand};
    }
  }
`;

export default AddMenu;
