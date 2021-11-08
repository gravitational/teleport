import React from 'react';
import { render, screen } from 'design/utils/testing';
import { Loaded, LoadedFirstTime, Failed } from './RecoveryCodesDialog.story';

test('render successful recovery codes dialog', () => {
  render(<Loaded />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render successful first timer recovery codes dialog', () => {
  render(<LoadedFirstTime />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render failed state of recovery codes dialog', () => {
  render(<Failed />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
