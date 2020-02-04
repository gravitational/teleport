import React from 'react';
import Button, { ButtonPrimary, ButtonSecondary, ButtonWarning } from './index';
import { render, theme } from 'design/utils/testing';

describe('design/Button', () => {
  it('renders a <button> and respects default "kind" prop == primary', () => {
    const { container } = render(<Button />);
    expect(container.firstChild.nodeName).toEqual('BUTTON');
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.secondary.main,
    });
  });

  test('"kind" primary renders bg == theme.colors.secondary.main', () => {
    const { container } = render(<ButtonPrimary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.secondary.main,
    });
  });

  test('"kind" secondary renders bg == theme.colors.primary.light', () => {
    const { container } = render(<ButtonSecondary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.primary.light,
    });
  });

  test('"kind" warning renders bg == theme.colors.error.dark', () => {
    const { container } = render(<ButtonWarning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.error.dark,
    });
  });

  test('"size" small renders min-height: 24px', () => {
    const { container } = render(<Button size="small" />);
    expect(container.firstChild).toHaveStyle({ 'min-height': '24px' });
  });

  test('"size" medium renders min-height: 40px', () => {
    const { container } = render(<Button size="medium" />);
    expect(container.firstChild).toHaveStyle('min-height: 40px');
  });

  test('"size" large renders min-height: 48px', () => {
    const { container } = render(<Button size="large" />);
    expect(container.firstChild).toHaveStyle('min-height: 48px');
  });

  test('"block" prop renders width 100%', () => {
    const { container } = render(<Button block />);
    expect(container.firstChild).toHaveStyle('width: 100%');
  });
});
