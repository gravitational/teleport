import React from 'react';
import { Story } from './SideNav.story';
import { render } from 'design/utils/testing';

test('rendering of SideNav', () => {
  const { container } = render(<Story />);
  expect(container.firstChild).toMatchSnapshot();
});
