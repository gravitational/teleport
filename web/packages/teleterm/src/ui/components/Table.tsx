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
        light: theme.colors.primary.dark,
        lighter: theme.colors.primary.light,
        main: theme.colors.primary.darker,
      },
      link: theme.colors.text.primary,
    },
  };
}
