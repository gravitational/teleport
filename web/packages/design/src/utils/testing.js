import React from 'react';
import {
  render as testingRender,
  screen,
  fireEvent,
} from '@testing-library/react';
import renderer from 'react-test-renderer';
import ThemeProvider from 'design/ThemeProvider';
import theme from 'design/theme';

export function render(component) {
  return testingRender(
    <ThemeProvider theme={theme}>{component}</ThemeProvider>
  );
}

export function convertToJSON(component) {
  return renderer
    .create(<ThemeProvider theme={theme}>{component}</ThemeProvider>)
    .toJSON();
}

export { screen, fireEvent };
