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
import { Box, Input, LabelInput } from 'design';

export default function FieldInputSsh({
  onPress,
  autoFocus = true,
  width = '200px',
  labelProps = {},
  ...boxProps
}) {
  const [hasError, setHasError] = React.useState(false);

  function onKeyPress(e) {
    const value = e.target.value;
    if ((e.key === 'Enter' || e.type === 'click') && value) {
      const valid = check(value);
      setHasError(!valid);
      if (valid) {
        const [login, serverId] = value.split('@');
        onPress(login, serverId);
      }
    } else {
      setHasError(false);
    }
  }

  const labelText = hasError ? 'Invalid' : 'Quick Launch';

  return (
    <Box {...boxProps}>
      <LabelInput {...labelProps} hasError={hasError}>
        {labelText}
      </LabelInput>
      <StyledInput
        height="34px"
        bg="primary.light"
        color="text.primary"
        placeholder="login@host"
        autoFocus={autoFocus}
        width={width}
        onKeyPress={onKeyPress}
      />
    </Box>
  );
}

const SSH_STR_REGEX = /(^(\w+-?\w+)+@(\S+)$)/;
const check = value => {
  const match = SSH_STR_REGEX.exec(value);
  return match !== null;
};

const StyledInput = styled(Input)(
  ({ theme }) => `
  background: ${theme.colors.primary.light};
  border: 1px solid ${theme.colors.primary.dark};
  border-radius: 4px;
  border-color: rgba(255, 255, 255, 0.24);

  &:hover, &:focus, &:active {
    color: ${theme.colors.text.primary};
    background: ${theme.colors.primary.lighter};
    box-shadow: inset 0 2px 4px rgba(0, 0, 0, .24);
  }

  font-size: ${theme.fontSizes[2]}px;
  font-family: ${theme.font};

  &::placeholder {
    color: ${theme.colors.text.placeholder};
    font-size: ${theme.fontSizes[2]}px;
  }
`
);
