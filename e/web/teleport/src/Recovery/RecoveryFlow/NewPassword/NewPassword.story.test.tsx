import React from 'react';
import { render } from 'design/utils/testing';
import { Loaded, Failed } from './NewPassword.story';

test('render correct form for resetting password', () => {
  const { container } = render(<Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render correct form for failed state for resetting password', () => {
  const { container } = render(<Failed />);

  expect(container.firstChild).toMatchSnapshot();
});
