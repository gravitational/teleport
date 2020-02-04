import React from 'react';
import ButtonIcon from './index';
import { render } from 'design/utils/testing';

describe('design/ButtonIcon', () => {
  it('renders a <button> and respects default "size" to 1', () => {
    const { container } = render(<ButtonIcon />);
    expect(container.firstChild.nodeName).toEqual('BUTTON');
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 0 maps to font-size 12px', () => {
    const { container } = render(<ButtonIcon size={0} />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  test('"size" 1 maps to font-size 16px', () => {
    const { container } = render(<ButtonIcon size={1} />);
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 2 maps to font-size 24px', () => {
    const { container } = render(<ButtonIcon size={2} />);
    expect(container.firstChild).toHaveStyle('font-size: 24px');
  });
});
