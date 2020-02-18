/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import ActionMenu, { MenuItem } from '.';

export default {
  title: 'Shared/Action Menu',
};

export const Basic = () => (
  <ActionMenu>
    <MenuItem>Edit...</MenuItem>
    <MenuItem>Delete...</MenuItem>
  </ActionMenu>
);

export const EmptyList = () => <ActionMenu />;

const inlineCss = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};
export const ButtonRightSide = () => (
  <ActionMenu buttonIconProps={inlineCss}>
    <MenuItem>Edit...</MenuItem>
    <MenuItem>Delete...</MenuItem>
  </ActionMenu>
);
