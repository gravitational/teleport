/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Component } from 'react';
import styled from 'styled-components';

import { color, height, space } from 'design/system';
import { debounce } from 'shared/utils/highbar';

class InputSearch extends Component {
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
        color="text.main"
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
    background: props.theme.colors.levels.sunken,

    '&:hover': {
      background: props.theme.colors.levels.elevated,
    },
    '&:focus, &:active': {
      background: props.theme.colors.levels.elevated,
      boxShadow: 'inset 0 2px 4px rgba(0, 0, 0, .24)',
      color: props.theme.colors.text.main,
    },
    '&::placeholder': {
      color: props.theme.colors.text.muted,
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
