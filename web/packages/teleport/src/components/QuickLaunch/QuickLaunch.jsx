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
import { Flex } from 'design';
import { space, width, color, height } from 'styled-system';

// Checks for spaces between chars, and
// captures two named groups: username and host.
const SSH_STR_REGEX = /^(?:(?<username>[^\s]+)@)(?<host>[^\s]+)$/;
const check = value => {
  return SSH_STR_REGEX.exec(value.trim());
};

export default function FieldInputSsh({
  onPress,
  autoFocus = false,
  inputProps = {},
  ...boxProps
}) {
  const [hasError, setHasError] = React.useState(false);

  function onKeyPress(e) {
    const value = e.target.value;
    if ((e.key === 'Enter' || e.type === 'click') && value) {
      const match = check(value);
      setHasError(!match);
      if (match) {
        const { username, host } = match.groups;
        onPress(username, host);
      }
    } else {
      setHasError(false);
    }
  }

  return (
    <StyledBox {...boxProps} hasError={hasError}>
      <StyledLabel>SSH:</StyledLabel>
      <StyledInput
        bg="levels.surface"
        color="text.main"
        placeholder="login@host:port"
        autoFocus={autoFocus}
        onKeyPress={onKeyPress}
        {...inputProps}
      />
    </StyledBox>
  );
}

function error({ hasError, theme }) {
  if (!hasError) {
    return;
  }

  return {
    border: `1px solid ${theme.colors.error.main}`,
    paddifngLeft: '7px',
    paddifngRight: '1px',
  };
}

const StyledBox = styled(Flex)`
  align-items: center;
  height: 32px;
  border: 1px solid;
  border-radius: 4px;
  border-color: rgba(255, 255, 255, 0.24);
  ${error}
`;

const StyledLabel = styled.div`
  opacity: 0.75;
  font-size: 11px;
  font-weight: 500;
  padding: 0 8px;
  border-bottom-left-radius: 4px;
  border-top-left-radius: 4px;
`;

const StyledInput = styled.input`
  appearance: none;
  border: none;
  border-radius: 4px;
  box-sizing: border-box;
  border-bottom-left-radius: unset;
  border-top-left-radius: unset;
  display: block;
  outline: none;
  width: 100%;
  height: 100%;
  box-shadow: none;
  padding-left: 8px;
  font-size: 12px;

  ::-ms-clear {
    display: none;
  }

  :read-only {
    cursor: not-allowed;
  }

  ::placeholder {
    opacity: 1;
    color: ${props => props.theme.colors.text.muted};
    font-size: ${props => props.theme.fontSizes[1]}px;
  }

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.levels.elevated};
  }

  ${color} ${space} ${width} ${height};
`;
