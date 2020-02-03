import React from 'react';
import ButtonLink from './index';
import { render } from 'design/utils/testing';

describe('Design/ButtonLink', () => {
  it('respects the "as" prop', () => {
    const { container } = render(<ButtonLink />);
    expect(container.firstChild.nodeName).toEqual('A');
  });

  it('respects the button size prop', () => {
    const { container } = render(<ButtonLink size="large" />);
    expect(container.firstChild).toHaveStyleRule('min-height', '48px');
  });
});
