import React from 'react';
import { Error, Warning, Info } from './LicenseWarning.story';
import { render, screen } from 'design/utils/testing';

test('render error license warning', async () => {
  render(<Error />);
  expect(screen.getByTestId('warning')).toMatchSnapshot();
});

test('render warning license warning', async () => {
  render(<Warning />);
  expect(screen.getByTestId('warning')).toMatchSnapshot();
});

test('render info license warning', async () => {
  render(<Info />);
  expect(screen.getByTestId('warning')).toMatchSnapshot();
});
