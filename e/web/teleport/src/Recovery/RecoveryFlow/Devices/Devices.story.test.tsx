import React from 'react';
import { render, screen } from 'design/utils/testing';
import { Loaded, Failed, RemoveDeviceDialog } from './Devices.story';

test('render device list', () => {
  const { container } = render(<Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state for device list', () => {
  const { container } = render(<Failed />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render device removal dialog', () => {
  render(<RemoveDeviceDialog />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
