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
import theme from 'design/theme';
import { ThemeProvider } from 'styled-components';

export const colors = {
  primary: {
    main: '#fff',
    light: '#fafafa',
    lighter: '#fafafa',
    dark: '#fafafa',
    contrastText: '#FFFFFF',
  },

  secondary: {
    main: '#512FC9',
    light: '#651FFF',
    dark: '#354AA4',
    contrastText: '#FFFFFF',
  },

  text: {
    // The most important text.
    primary: 'rgba(0, 0, 0, 0.87)',
    // Secondary text.
    secondary: 'rgba(0, 0, 0, 0.56)',
    // Placeholder text for forms.
    placeholder: 'rgba(0, 0, 0, 0.24)',
    // Disabled text have even lower visual prominence.
    disabled: 'rgba(0, 0, 0, 0.24)',
    // Text hints.
    hint: 'rgba(0, 0, 0, 0.24)',
    // On light backgrounds
    onLight: '#324148',
    // On dark backgrounds
    onDark: 'rgba(255, 255, 255, 0.87)',
  },

  action: {
    active: '#00000',
    hover: 'rgba(0, 0, 0, 0.1)',
    hoverOpacity: 0.1,
    selected: 'rgba(0, 0, 0, 0.2)',
    disabled: 'rgba(0, 0, 0, 0.3)',
    disabledBackground: 'rgba(0, 0, 0, 0.12)',
  },
};

const customTheme = {
  ...theme,
};

customTheme.colors = {
  ...theme.colors,
  primary: {
    ...theme.colors.primary,
    ...colors.primary,
  },
  secondary: {
    ...theme.colors.secondary,
    ...colors.secondary,
  },
  text: {
    ...theme.colors.text,
    ...colors.text,
  },
  action: {
    ...theme.colors.action,
    ...colors.action,
  },
};

const LightThemeProvider = props => (
  <ThemeProvider theme={customTheme} children={props.children} />
);

export default LightThemeProvider;
