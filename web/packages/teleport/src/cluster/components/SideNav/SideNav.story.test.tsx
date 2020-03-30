import React from 'react';
import { SideNav } from './SideNav.story';
import { render } from 'design/utils/testing';

test('rendering of SideNav', () => {
  const { container } = render(<SideNav />);
  expect(container.firstChild).toMatchSnapshot();
});
