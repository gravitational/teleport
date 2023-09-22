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

import { space, color, alignSelf } from 'design/system';

import Icon from '../Icon';

const sizeMap = {
  0: {
    fontSize: '12px',
    height: '24px',
    width: '24px',
  },
  1: {
    fontSize: '16px',
    height: '32px',
    width: '32px',
  },
  2: {
    fontSize: '24px',
    height: '48px',
    width: '48px',
  },
};

const defaultSize = sizeMap[1];

const size = props => {
  return sizeMap[props.size] || defaultSize;
};

const fromProps = props => {
  const { theme } = props;
  return {
    '&:disabled': {
      color: theme.colors.text.disabled,
      cursor: 'default',
    },
    '&:hover:enabled, &:focus:enabled': {
      background: theme.colors.spotBackground[1],
    },
    '&:active:enabled': {
      background: theme.colors.spotBackground[2],
    },
  };
};

const ButtonIcon = props => {
  const { children, setRef, ...rest } = props;
  return (
    <StyledButtonIcon ref={setRef} {...rest}>
      {children}
    </StyledButtonIcon>
  );
};

const StyledButtonIcon = styled.button`
  align-items: center;
  border: none;
  cursor: pointer;
  display: flex;
  outline: none;
  border-radius: 50%;
  overflow: visible;
  justify-content: center;
  text-align: center;
  flex: 0 0 auto;
  background: transparent;
  color: inherit;
  transition: all 0.3s;
  -webkit-font-smoothing: antialiased;

  ${Icon} {
    color: inherit;
  }

  &:disabled {
    color: ${({ theme }) => theme.colors.text.disabled};
  }

  ${fromProps}
  ${size}
  ${space}
  ${color}
  ${alignSelf}
`;
export default ButtonIcon;
