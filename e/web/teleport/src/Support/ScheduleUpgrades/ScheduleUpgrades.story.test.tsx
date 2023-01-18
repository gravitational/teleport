import React from 'react';
import { render, screen } from 'design/utils/testing';

import { Loaded, Processing, Failed } from './ScheduleUpgrades.story';

test('render loaded', () => {
  render(<Loaded />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render processing', () => {
  render(<Processing />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render failed', () => {
  render(<Failed />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
