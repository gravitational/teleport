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
import Menu, { MenuItem} from 'design/Menu';

class MenuFilter extends React.Component {

  static displayName = 'MenuFilter';

  static defaultProps = {
    menuListCss: () => { },
  }

  constructor(props){
    super(props)
    this.state = {
      open: Boolean(props.open),
    }
  }

  onOpen = () => {
    this.setState({ open: true }, () => {
      this.props.onOpen && this.props.onOpen();
    });

  };

  onClose = () => {
    this.setState({ open: false });
  }

  onPod = () => {
    this.setState({ open: false }, () =>
      this.props.onPod && this.props.onPod());
  }

  onContainer = () => {
    this.setState({ open: false }, () =>
      this.props.onContainer && this.props.onContainer());
  }

  render() {
    const {
      anchorOrigin,
      transformOrigin,
    } = this.props;

    const { open } = this.state;

    return (
      <React.Fragment>
        <button ref={e => this.anchorEl = e } onClick={this.onOpen}>
          Filter
        </button>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={this.anchorEl}
          open={Boolean(open)}
          onClose={this.onClose}
        >
          <MenuItem onClick={this.onContainer}>
            Containers
          </MenuItem>
          <MenuItem onClick={this.onPod}>
            Pods
          </MenuItem>
        </Menu>
      </React.Fragment>
    );
  }
}

export default MenuFilter;