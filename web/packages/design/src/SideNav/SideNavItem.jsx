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
    fontSize: theme.fontSizes[1],
    fontWeight: theme.bold,
    '&:active, &.active': {
      background: theme.colors.primary.light,
      borderLeftColor: theme.colors.accent,
      color: theme.colors.primary.contrastText,
    },
    '&:hover': {
      background: theme.colors.primary.light,
    },
  };
};

const SideNavItem = styled(Flex)`
  min-height: 72px;
  align-items: center;
  cursor: pointer;
  justify-content: flex-start;
  outline: none;
  text-decoration: none;
  width: 100%;
  border-left: 4px solid transparent;
  ${fromTheme}
  ${borderColor}
`;

SideNavItem.displayName = 'SideNavItem';

SideNavItem.defaultProps = {
  pl: '10',
  pr: '5',
  bg: 'primary.main',
  color: 'text.primary',
};

export default SideNavItem;
