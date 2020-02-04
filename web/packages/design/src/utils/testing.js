import React from 'react';
import {
  render as testingRender,
  act,
  fireEvent,
  waitForElement,
} from '@testing-library/react';
import { screen, wait, prettyDOM } from '@testing-library/dom';
import ThemeProvider from 'design/ThemeProvider';
import theme from 'design/theme';
import '@testing-library/jest-dom';
import 'jest-styled-components';

function Providers({ children }) {
  return <ThemeProvider theme={theme}>{children}</ThemeProvider>;
}

function render(ui, options) {
  return testingRender(ui, { wrapper: Providers, ...options });
}

screen.debug = () => {
  window.console.log(prettyDOM());
};

export {
  act,
  screen,
  wait,
  fireEvent,
  theme,
  render,
  prettyDOM,
  waitForElement,
};
