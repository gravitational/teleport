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

import { borderColor } from './../system';
import defaultTheme from './../theme';
import Flex from './../Flex';

const fromTheme = ({ theme = defaultTheme }) => {
  return {
    background: theme.colors.levels.surface,
    color: theme.colors.text.secondary,
    fontSize: theme.fontSizes[1],
    fontWeight: theme.bold,
    '&:active, &.active': {
      borderLeftColor: theme.colors.brand.accent,
      background: theme.colors.levels.elevated,
      color: theme.colors.text.contrast,
    },
    '&:hover, &:focus': {
      background: theme.colors.levels.elevated,
      color: theme.colors.text.contrast,
    },
  };
};

const SideNavItem = styled(Flex)`
  min-height: 56px;
  align-items: center;
  justify-content: flex-start;
  border-left: 4px solid transparent;
  cursor: pointer;
  outline: none;
  text-decoration: none;
  width: 100%;
  ${fromTheme}
  ${borderColor}
`;

SideNavItem.displayName = 'SideNavItem';

SideNavItem.defaultProps = {
  pl: 9,
  pr: 5,
  bg: 'levels.surfaceSecondary',
  color: 'text.primary',
  theme: defaultTheme,
};

export default SideNavItem;
