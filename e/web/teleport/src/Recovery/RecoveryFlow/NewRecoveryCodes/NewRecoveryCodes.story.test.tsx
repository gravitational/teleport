import React from 'react';
import { render } from 'design/utils/testing';
import { Loaded, Failed } from './NewRecoveryCodes.story';

test('render recovery codes', () => {
  const { container } = render(<Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state for recovery codes', () => {
  const { container } = render(<Failed />);

  expect(container.firstChild).toMatchSnapshot();
});
