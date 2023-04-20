/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { FC } from 'react';
import { ThemeProvider } from 'styled-components';

import theme from 'teleterm/ui/ThemeProvider/theme';

const menuLoginTheme = {
  ...theme,
  colors: {
    ...theme.colors,
    subtle: theme.colors.levels.elevated,
    light: theme.colors.levels.surface,
    levels: {
      ...theme.colors.levels,
      sunkenSecondary: theme.colors.text.primary,
    },
    grey: {
      [50]: 'rgba(255,255,255,0.05)',
      [900]: theme.colors.text.primary,
      [100]: theme.colors.text.secondary,
    },
    link: theme.colors.text.primary,
  },
};

export const MenuLoginTheme: FC = props => (
  <ThemeProvider theme={menuLoginTheme} children={props.children} />
);
