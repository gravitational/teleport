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
import { Box } from 'design';

import { ToolTipNoPermBadge } from './ToolTipNoPermBadge';

export default {
  title: 'Teleport/ToolTip',
};

export const NoPermissionBadgeString = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge children={'I am a string'} />
  </SomeBox>
);

export const NoPermissionBadgeComp = () => (
  <SomeBox>
    I'm a sample container
    <ToolTipNoPermBadge
      children={<Box p={3}>I'm a box component with too much padding</Box>}
    />
  </SomeBox>
);

const SomeBox = styled.div`
  width: 240px;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  position: relative;
  align-items: center;
  background-color: ${props => props.theme.colors.spotBackground[0]};
`;
