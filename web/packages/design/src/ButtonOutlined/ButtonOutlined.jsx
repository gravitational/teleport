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

import { space, width } from 'design/system';
import defaultTheme from 'design/theme';

const ButtonOutlined = (
  { children, setRef, ...props } = { setRef: undefined }
) => {
  return (
    <StyledButton {...props} ref={setRef}>
      <span>{children}</span>
    </StyledButton>
  );
};

const size = props => {
  switch (props.size) {
    case 'small':
      return {
        fontSize: '10px',
        minHeight: '24px',
        padding: '0px 16px',
      };
    case 'large':
      return {
        minHeight: '40px',
        fontSize: '12px',
        padding: '0px 40px',
      };
    default:
      // medium
      return {
        minHeight: '32px',
        fontSize: `12px`,
        padding: '0px 24px',
      };
  }
};

const themedStyles = props => {
  const { colors } = props.theme;
  const style = {
    color: colors.text.contrast,
    '&:disabled': {
      background: colors.action.disabledBackground,
      color: colors.action.disabled,
    },
  };

  return {
    ...kinds(props),
    ...style,
    ...size(props),
    ...space(props),
    ...width(props),
    ...block(props),
  };
};

const kinds = props => {
  const { kind, theme } = props;
  switch (kind) {
    case 'primary':
      return {
        borderColor: theme.colors.buttons.outlinedPrimary.border,
        color: theme.colors.buttons.outlinedPrimary.text,
        '&:hover, &:focus': {
          borderColor: theme.colors.buttons.outlinedPrimary.borderHover,
        },
        '&:active': {
          borderColor: theme.colors.buttons.outlinedPrimary.borderActive,
        },
      };
    default:
      return {
        borderColor: theme.colors.buttons.outlinedDefault.border,
        color: theme.colors.buttons.outlinedDefault.text,
        '&:hover, &:focus': {
          borderColor: theme.colors.buttons.outlinedDefault.borderHover,
          color: theme.colors.buttons.outlinedDefault.textHover,
        },
      };
  }
};

const block = props =>
  props.block
    ? {
        width: '100%',
      }
    : null;

const StyledButton = styled.button`
  line-height: 1.5;
  border-radius: 4px;
  display: inline-flex;
  justify-content: center;
  align-items: center;
  border: 1px solid;
  box-sizing: border-box;
  background-color: transparent;
  cursor: pointer;
  font-family: inherit;
  font-weight: bold;
  outline: none;
  opacity: 0.56;
  position: relative;
  text-align: center;
  text-decoration: none;
  text-transform: uppercase;
  transition: all 0.3s;
  -webkit-font-smoothing: antialiased;

  &:hover {
    opacity: 1;
  }

  &:active {
    opacity: 0.24;
  }

  > span {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  ${themedStyles}
  ${kinds}
  ${block}
`;

ButtonOutlined.propTypes = {
  ...space.propTypes,
};

ButtonOutlined.defaultProps = {
  size: 'medium',
  theme: defaultTheme,
};

ButtonOutlined.displayName = 'ButtonOutlined';

export default ButtonOutlined;
