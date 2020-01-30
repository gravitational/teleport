import React from 'react';
import Alert, { Danger, Info, Warning, Success } from './index';
import { render, theme } from 'design/utils/testing';

describe('Design/Alert', () => {
  it('respects default "kind" prop == danger', () => {
    const { container } = render(<Alert />);
    expect(container.firstChild).toHaveStyleRule(
      'background',
      theme.colors.danger
    );
  });

  test('"kind" danger renders bg == theme.colors.danger', () => {
    const { container } = render(<Danger />);
    expect(container.firstChild).toHaveStyleRule(
      'background',
      theme.colors.danger
    );
  });

  test('"kind" warning renders bg == theme.colors.warning', () => {
    const { container } = render(<Warning />);
    expect(container.firstChild).toHaveStyleRule(
      'background',
      theme.colors.warning
    );
  });

  test('"kind" info renders bg == theme.colors.info', () => {
    const { container } = render(<Info />);
    expect(container.firstChild).toHaveStyleRule(
      'background',
      theme.colors.info
    );
  });

  test('"kind" success renders bg == theme.colors.success', () => {
    const { container } = render(<Success />);
    expect(container.firstChild).toHaveStyleRule(
      'background',
      theme.colors.success
    );
  });
});
