import React from 'react';
import { Loaded } from './Sessions.story';
import { render } from 'design/utils/testing';

test('loaded', () => {
  const { container } = render(<Loaded />);
  expect(container.firstChild).toMatchSnapshot();
});
