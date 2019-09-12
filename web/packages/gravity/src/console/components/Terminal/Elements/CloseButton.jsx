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
import { Close as CloseIcon } from 'design/Icon';
import { space } from 'styled-system';

export default function CloseButton(props) {
  return (
    <StyledCloseButton title="Close" {...props}>
      <CloseIcon />
    </StyledCloseButton>
  )
}

const StyledCloseButton = styled.button`
  background: ${props => props.theme.colors.error.dark};
  border-radius: 2px;
  border: none;
  cursor: pointer;
  height: 16px;
  outline: none;
  padding: 0;
  width: 16px;
  &:hover {
    background: ${props => props.theme.colors.error};
  }
  ${space}
`
