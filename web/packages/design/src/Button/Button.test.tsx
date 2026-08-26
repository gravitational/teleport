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

import { render, screen } from 'design/utils/testing';

import { Button, ButtonPrimary, ButtonSize } from './index';

describe('design/Button', () => {
  test('"size" small renders min-height: 24px', () => {
    render(<Button size="small">Hi</Button>);
    expect(screen.getByRole('button')).toHaveStyle({ 'min-height': '24px' });
  });

  test('"size" medium renders min-height: 32px', () => {
    render(<Button size="medium">Hi</Button>);
    expect(screen.getByRole('button')).toHaveStyle('min-height: 32px');
  });

  test('"size" large renders min-height: 40px', () => {
    render(<Button size="large">Hi</Button>);
    expect(screen.getByRole('button')).toHaveStyle('min-height: 40px');
  });

  test('"block" prop renders width 100%', () => {
    render(<Button block>Hi</Button>);
    expect(screen.getByRole('button')).toHaveStyle('width: 100%');
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

    // For some reason, ComponentPropsWithoutRef or ComponentProps cause types for regular props
    // such as onClick to not be correctly recognized, hence the use of ComponentPropsWithRef.
    const RegularWrapper = <Element extends React.ElementType = 'button'>(
      props: ComponentPropsWithRef<typeof ButtonPrimary<Element>>
    ) => {
      return <ButtonPrimary size="large" {...props} />;
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
      return typeof foo + 'bar';
    };
  });
});
