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
import { space } from 'design/system';
import { Close as CloseIcon } from 'design/Icon';

export default function ButtonClose(props) {
  return (
    <StyledCloseButton title="Close" {...props}>
      <CloseIcon />
    </StyledCloseButton>
  );
}

const StyledCloseButton = styled.button`
  background: #0000;
  border-radius: 2px;
  border: none;
  color: #fff;
  cursor: pointer;
  height: 20px;
  opacity: 0.56;
  outline: none;
  padding: 0;
  position: absolute;
  right: 8px;
  top: 8px;
  transition: all 0.3s;
  width: 20px;
  &:hover {
    opacity: 1;
  }

  &:hover {
    background: ${props => props.theme.colors.error};
  }
  font-size: ${props => props.theme.fontSizes[4]}px;

  ${space}
`;
