import React from 'react';
import ButtonOutlined from './index';
import { render, theme } from 'design/utils/testing';

describe('design/ButtonOutlined', () => {
  it('renders a <button> and respects default props', () => {
    const { container } = render(<ButtonOutlined />);
    expect(container.firstChild.nodeName).toEqual('BUTTON');
    expect(container.firstChild).toHaveStyle('font-size: 12px');
    expect(container.firstChild).toHaveStyle({
      'border-color': theme.colors.text.primary,
    });
  });

  it('respects "size" small prop', () => {
    const { container } = render(<ButtonOutlined size="small" />);
    expect(container.firstChild).toHaveStyle('font-size: 10px');
  });

  it('respects "size" medium prop', () => {
    const { container } = render(<ButtonOutlined size="medium" />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  it('respects "size" large prop', () => {
    const { container } = render(<ButtonOutlined size="large" />);
    expect(container.firstChild).toHaveStyle('font-size: 14px');
  });

  it('respects "kind" primary prop', () => {
    const { container } = render(<ButtonOutlined kind="primary" />);
    expect(container.firstChild).toHaveStyle({
      'border-color': theme.colors.secondary.main,
    });
  });
});
