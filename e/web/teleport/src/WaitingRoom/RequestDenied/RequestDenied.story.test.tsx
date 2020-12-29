import React from 'react';
import { WithReason, WithoutReason } from './RequestDenied.story';
import { render, screen } from 'design/utils/testing';

test('loaded with denied reason', () => {
  render(<WithReason />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('loaded without denied reason', () => {
  render(<WithoutReason />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
