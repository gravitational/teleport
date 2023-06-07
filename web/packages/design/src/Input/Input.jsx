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

import styled from 'styled-components';
import PropTypes from 'prop-types';
import { space, width, color, height } from 'styled-system';

function error({ hasError, theme }) {
  if (!hasError) {
    return;
  }

  return {
    border: `2px solid ${theme.colors.error.main}`,
    '&:hover, &:focus': {
      border: `2px solid ${theme.colors.error.main}`,
    },
    padding: '10px 14px',
  };
}

const Input = styled.input`
  appearance: none;
  border: 1px solid ${props => props.theme.colors.text.muted};
  border-radius: 4px;
  box-sizing: border-box;
  display: block;
  height: 40px;
  font-size: 16px;
  padding: 0 16px;
  outline: none;
  width: 100%;
  background: ${props => props.theme.colors.levels.surface};
  color: ${props => props.theme.colors.text.main};

  &:hover,
  &:focus,
  &:active {
    border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  }

  ::-ms-clear {
    display: none;
  }

  ::placeholder {
    color: ${props => props.theme.colors.text.muted};
    opacity: 1;
  }

  :read-only {
    cursor: not-allowed;
  }

  :disabled {
    color: ${props => props.theme.colors.text.disabled};
    border-color: ${props => props.theme.colors.text.disabled};
  }

  ${color} ${space} ${width} ${height} ${error};
`;

Input.displayName = 'Input';

Input.propTypes = {
  placeholder: PropTypes.string,
  hasError: PropTypes.bool,
};

export default Input;
