import React from 'react';
import { Sample } from './TopNav.story';
import { render } from 'design/utils/testing';

describe('design/TopNav', () => {
  it('should render', () => {
    const { container } = render(<Sample />);
    expect(container.querySelectorAll('nav')).toHaveLength(1);
    expect(container.querySelectorAll('button')).toHaveLength(3);
  });
});
