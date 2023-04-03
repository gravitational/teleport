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
import { debounce } from 'shared/utils/highbar';
import styled from 'styled-components';
import { height, space, color } from 'design/system';

class InputSearch extends React.Component {
  constructor(props) {
    super(props);
    this.debouncedNotify = debounce(() => {
      this.props.onChange(this.state.value);
    }, 200);

    let value = props.value || '';

    this.state = {
      value,
      isFocused: false,
    };
  }

  onBlur = () => {
    this.setState({ isFocused: false });
  };

  onFocus = () => {
    this.setState({ isFocused: true });
  };

  onChange = e => {
    this.setState({ value: e.target.value });
    this.debouncedNotify();
  };

  render() {
    const { autoFocus = false, ...rest } = this.props;
    return (
      <Input
        px="3"
        placeholder="SEARCH..."
        color="text.primary"
        {...rest}
        autoFocus={autoFocus}
        value={this.state.value}
        onChange={this.onChange}
        onFocus={this.onFocus}
        onBlur={this.onBlur}
      />
    );
  }
}

function fromTheme(props) {
  return {
    background: props.theme.colors.levels.sunkenSecondary,

    '&:hover': {
      background: props.theme.colors.levels.elevated,
    },
    '&:focus, &:active': {
      background: props.theme.colors.levels.elevated,
      boxShadow: 'inset 0 2px 4px rgba(0, 0, 0, .24)',
      color: props.theme.colors.text.primary,
    },
    '&::placeholder': {
      color: props.theme.colors.text.placeholder,
      fontSize: props.theme.fontSizes[1],
    },
  };
}

const Input = styled.input`
  box-sizing: border-box;
  font-size: 12px;
  min-width: 200px;
  outline: none;
  border: none;
  border-radius: 200px;
  height: 32px;
  transition: all 0.2s;
  ${fromTheme}
  ${space}
  ${color}
  ${height}
`;

export default InputSearch;
