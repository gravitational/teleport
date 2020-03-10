import React from 'react';
import { Sample } from './TopNav.story';
import { render } from 'design/utils/testing';

test('rendering of TopNav and TopNavItem', () => {
  const { container } = render(<Sample />);
  expect(container.firstChild).toMatchSnapshot();
});
