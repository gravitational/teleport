import React, { FC } from 'react';
import { ThemeProvider } from 'styled-components';
import theme from 'teleterm/ui/ThemeProvider/theme';

const menuSshLoginTheme = {
  ...theme,
  colors: {
    ...theme.colors,
    subtle:theme.colors.primary.lighter,
    light: theme.colors.primary.dark,
    grey: {
      [50]: 'rgba(255,255,255,0.05)',
      [900]: theme.colors.text.primary,
    },
    link: theme.colors.text.primary,
  },
};

export const MenuSshLoginTheme: FC = props => (
  <ThemeProvider theme={menuSshLoginTheme} children={props.children} />
);
