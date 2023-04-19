import React from 'react';

import { render } from 'design/utils/testing';

import { Sample } from './TopNav.story';

test('rendering of TopNav and TopNavItem', () => {
  const { container } = render(<Sample />);
  expect(container.firstChild).toMatchSnapshot();
});
