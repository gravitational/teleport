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
import defaultTheme from 'design/theme';
import { color } from 'design/system';

const fromTheme = ({ $nested = false, theme = defaultTheme }) => {
  const css = {
    position: 'relative',
    fontSize: '12px',
    fontWeight: theme.regular,
    fontFamily: theme.font,
    paddingLeft: theme.space[9] + 'px',
    paddingRight: theme.space[5] + 'px',
    background: theme.colors.levels.surface,
    color: theme.colors.text.secondary,
    minHeight: '56px',

    '&:active, &.active': {
      borderLeftColor: $nested ? 'none' : theme.colors.brand.accent,
      background: theme.colors.levels.elevated,
      color: theme.colors.text.contrast,
      fontWeight: theme.bold,

      '.marker': {
        background: theme.colors.brand.accent,
      },
    },

    '&:hover': {
      background: theme.colors.levels.elevated,
    },
    '&:focus, &:hover': {
      color: theme.colors.text.contrast,
    },
  };

  if ($nested) {
    css.fontSize = '11px';
    css.background = 'none';
    css.fontWeight = theme.regular;
    css.paddingLeft = '96px';
    css.minHeight = '40px';
  }

  return css;
};

const SideNavItem = styled.div`
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  border: none;
  border-left: 4px solid transparent;
  cursor: pointer;
  outline: none;
  text-decoration: none;
  width: 100%;
  line-height: 24px;
  ${fromTheme}
  ${color}
`;

export default SideNavItem;
