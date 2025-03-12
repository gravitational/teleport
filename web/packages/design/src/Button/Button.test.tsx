/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, {
  ComponentPropsWithoutRef,
  ComponentPropsWithRef,
  PropsWithChildren,
} from 'react';

import { render, screen, theme } from 'design/utils/testing';

import {
  Button,
  ButtonPrimary,
  ButtonSecondary,
  ButtonSize,
  ButtonWarning,
} from './index';

describe('design/Button', () => {
  it('renders a <button> and respects default "kind" prop == primary', () => {
    const { container } = render(<Button />);
    expect(container.firstChild?.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.brand,
    });
  });

  test('"kind" primary renders bg == theme.colors.buttons.primary.default', () => {
    const { container } = render(<ButtonPrimary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.primary.default,
    });
  });

  test('"kind" secondary renders bg == theme.colors.buttons.secondary.default', () => {
    const { container } = render(<ButtonSecondary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.secondary.default,
    });
  });

  test('"kind" warning renders bg == theme.colors.buttons.warning.default', () => {
    const { container } = render(<ButtonWarning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.warning.default,
    });
  });

  test('"size" small renders min-height: 24px', () => {
    const { container } = render(<Button size="small" />);
    expect(container.firstChild).toHaveStyle({ 'min-height': '24px' });
  });

  test('"size" medium renders min-height: 32px', () => {
    const { container } = render(<Button size="medium" />);
    expect(container.firstChild).toHaveStyle('min-height: 32px');
  });

  test('"size" large renders min-height: 40px', () => {
    const { container } = render(<Button size="large" />);
    expect(container.firstChild).toHaveStyle('min-height: 40px');
  });

  test('"block" prop renders width 100%', () => {
    const { container } = render(<Button block />);
    expect(container.firstChild).toHaveStyle('width: 100%');
  });

  describe('types and as prop', () => {
    test('in regular components', () => {
      render(
        <>
          <ButtonPrimary
            // @ts-expect-error href does not exist on <button>.
            href="https://example.com"
          >
            Button
          </ButtonPrimary>

          <ButtonPrimary
            as="a"
            href="https://example.com"
            // @ts-expect-error onClick should accept a mouse event as the first arg.
            onClick={invalidOnClickHandler}
          >
            Link
          </ButtonPrimary>
        </>
      );

      expect(screen.getByText('Button').localName).toEqual('button');
      expect(screen.getByText('Link').localName).toEqual('a');
    });

    test('in wrappers with as prop', () => {
      render(
        <>
          <RegularWrapper
            // @ts-expect-error href does not exist on <button>.
            href="https://example.com"
          >
            Button
          </RegularWrapper>

          <RegularWrapper
            as="a"
            href="https://example.com"
            // @ts-expect-error onClick should accept a mouse event as the first arg.
            onClick={invalidOnClickHandler}
          >
            Link
          </RegularWrapper>
        </>
      );

      expect(screen.getByText('Button').localName).toEqual('button');
      expect(screen.getByText('Link').localName).toEqual('a');
    });

    test('in wrappers with forwardedAs prop', () => {
      render(
        <>
          <ForwardedAsWrapper
            // @ts-expect-error href does not exist on <button>.
            href="https://example.com"
          >
            Button
          </ForwardedAsWrapper>

          <ForwardedAsWrapper
            forwardedAs="a"
            href="https://example.com"
            // @ts-expect-error onClick should accept a mouse event as the first arg.
            onClick={invalidOnClickHandler}
          >
            Link
          </ForwardedAsWrapper>
        </>
      );

      expect(screen.getByText('Button').localName).toEqual('button');
      expect(screen.getByText('Link').localName).toEqual('a');
    });

    const RegularWrapper = <Element extends React.ElementType = 'button'>(
      props: PropsWithChildren<{
        size?: ButtonSize;
      }> &
        // For some reason, ComponentPropsWithoutRef or ComponentProps cause types for regular props
        // such as onClick to not be correctly recognized.
        ComponentPropsWithRef<typeof ButtonPrimary<Element>>
    ) => {
      const { size, children, ...buttonProps } = props;

      return (
        <ButtonPrimary size={size} {...buttonProps}>
          {children}
        </ButtonPrimary>
      );
    };

    const ForwardedAsWrapper = <Element extends React.ElementType = 'button'>(
      props: PropsWithChildren<{
        size?: ButtonSize;
        forwardedAs?: Element;
      }> &
        // With forwardedAs, WithRef or WithoutRef doesn't seem to make a difference, so we opt for
        // the variant without the ref.
        ComponentPropsWithoutRef<typeof ButtonPrimary<Element>>
    ) => {
      const { size, children, ...buttonProps } = props;

      return (
        <ButtonPrimary
          // The css prop causes the component to require "forwardedAs" instead of just "as".
          css={`
            border-top-bottom-radius: 0;
          `}
          size={size}
          {...buttonProps}
        >
          {children}
        </ButtonPrimary>
      );
    };

    const invalidOnClickHandler = (foo: string) => {
      return typeof foo;
    };
  });
});
