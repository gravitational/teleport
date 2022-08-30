import React from 'react';

import { render } from 'design/utils/testing';

import { Story } from './SideNav.story';

test('rendering of SideNav', () => {
  const { container } = render(<Story />);
  expect(container.firstChild).toMatchSnapshot();
});
