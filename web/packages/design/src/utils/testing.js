import React from 'react';
import {
  render as testingRender,
  screen,
  fireEvent,
} from '@testing-library/react';
import ThemeProvider from 'design/ThemeProvider';
import theme from 'design/theme';
import 'jest-styled-components';

export function render(component) {
  return testingRender(
    <ThemeProvider theme={theme}>{component}</ThemeProvider>
  );
}

export { screen, fireEvent, theme };
