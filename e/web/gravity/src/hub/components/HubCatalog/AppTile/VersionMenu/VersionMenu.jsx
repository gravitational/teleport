import React from 'react';
import styled from 'styled-components';
import { ButtonOutlined } from 'design';
import Menu, { MenuItem} from 'design/Menu';
import * as Icons from 'design/Icon';

class VersionMenu extends React.Component {

  static displayName = 'VersionMenu';

  static defaultProps = {
    menuListCss: () => { },
  }

  constructor(props){
    super(props)
    this.state = {
      open: false,
      anchorEl: null,
    }
  }

  onOpen = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  }

  setRef = e => {
    this.anchorEl = e;
  }

  onClick = value => {
    this.props.onChange(value)
    this.onClose();
  }

  render() {
    const {
      value,
      options,
      anchorOrigin,
      transformOrigin,
      ...styles
    } = this.props;

    const { open } = this.state;

    return (
      <React.Fragment>
        <StyledButton width="140px" size="small" py="1" setRef={this.setRef} onClick={this.onOpen} {...styles}>
          {`v.  ${value}`}
          <Icons.CarrotDown ml="2" fontSize="2" color="text.onDark"/>
        </StyledButton>
        <Menu
          menuListCss={menuListCss}
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
        >
          { open && this.renderItems(options)}
        </Menu>
      </React.Fragment>
    );
  }

  renderItems(options) {
    const $items = options.map(val => (
      <MenuItem key={val} onClick={() => this.onClick(val)}>
        {val}
      </MenuItem>
    ));

    return $items;
  }
}

const menuListCss = props => `
  min-width: 140px;
  display: flex;
  flex-direction: column;
  ${MenuItem} {
    color: ${props.theme.colors.link};
  }
`

const StyledButton = styled(ButtonOutlined)`
  border: 1px solid;
  border-color: ${ ({theme}) => theme.colors.primary.main };
  > span {
    width: 100%;
    justify-content: space-between;
  }
`

export default VersionMenu;