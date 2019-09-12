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
import Button from 'design/Button';
import { isObject } from 'lodash';
import * as Icons from 'design/Icon';

class SelectNamespace extends React.Component {
  constructor(props){
    super(props)
    this.state = {
      open: Boolean(props.open),
      anchorEl: null,
    }
  }

  onOpen = e => {
    e.stopPropagation();
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
  }

  setRef = e => {
    this.anchorEl = e;
  }

  findTitle(value, options) {
    const selected = options.find(o => o.value === value);
    if(selected){
      return selected.title || selected.value;
    }

    return 'Unknown';
  }

  render() {
    const { open } = this.state;
    const { value, options, disabled } = this.props;
    const selectOptions = formatOptions(options);
    const displayValue = this.findTitle(value, selectOptions);
    const $options = this.renderItems(selectOptions, value);
    return (
      <React.Fragment>
        <StyledButton
          px="3"
          disabled={disabled}
          setRef={this.setRef}
          onClick={this.onOpen}
        >
          {displayValue}
          <Icons.CarrotDown ml="3" fontSize="3" color="text.onDark"/>
        </StyledButton>
        <Menu
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
          menuListCss={menuListCss}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
        >
          {$options}
        </Menu>
      </React.Fragment>
    );
  }

  renderItems(options, open) {
    if(!open){
      return;
    }

    const items = options.map((o, index) => {
      const title = o.title || o.value;
      return (
        <MenuItem key={index} onClick={this.makeOnClick(o)}>
          {title}
        </MenuItem>
      )
    })

    return items;
  }

  makeOnClick(option){
    return e => {
      e.stopPropagation();
      this.onClose();
      this.props.onChange(option.value);
    }
  }
}

function formatOptions(options){
  options = options || [];
  return options.map(o => !isObject(o) ? makeOption(o) : o);
}

function makeOption(value, title){
  title = title || value;
  return {
    value,
    title
  }
}

const menuListCss = () => `
  width: 220px;
`

const StyledButton = styled(Button)`
  background: ${({ theme }) => theme.colors.primary.main};
  text-transform: initial;
  &:hover, &:focus {
    background: ${({ theme }) => theme.colors.primary.light};
  }

  &:active {
    background: ${({ theme }) => theme.colors.primary.light};
    opacity: .56;
  }
`

export default SelectNamespace;

