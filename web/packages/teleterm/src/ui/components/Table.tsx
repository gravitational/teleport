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

import React from 'react';
import { ThemeProvider, useTheme } from 'styled-components';
import DesignTable from 'design/DataTable/Table';

export const Table: typeof DesignTable = props => {
  const theme = useTheme();

  return (
    <ThemeProvider theme={getTableTheme(theme)}>
      <DesignTable {...props} />
    </ThemeProvider>
  );
};

function getTableTheme(theme) {
  return {
    ...theme,
    colors: {
      ...theme.colors,
      primary: {
        ...theme.colors.primary,
        dark: 'rgba(255, 255, 255, 0.05)',
        light: theme.colors.levels.sunkenSecondary,
        lighter: theme.colors.levels.surface,
        main: theme.colors.levels.sunken,
      },
      link: theme.colors.text.primary,
    },
  };
}
