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

import React from 'react';

import { render, screen, theme, userEvent } from 'design/utils/testing';

import { Alert, Banner } from '.';

describe('Alert', () => {
  test.each`
    kind         | background
    ${undefined} | ${theme.colors.interactive.tonal.danger[0].background}
    ${'neutral'} | ${theme.colors.interactive.tonal.neutral[0].background}
    ${'danger'}  | ${theme.colors.interactive.tonal.danger[0].background}
    ${'warning'} | ${theme.colors.interactive.tonal.alert[0].background}
    ${'info'}    | ${theme.colors.interactive.tonal.informational[0].background}
    ${'success'} | ${theme.colors.interactive.tonal.success[0].background}
  `('renders appropriate background for kind $kind', ({ kind, background }) => {
    const { container } = render(<Alert kind={kind} />);
    expect(container.firstChild.firstChild).toHaveStyle({ background });
  });

  test('action buttons', async () => {
    const user = userEvent.setup();
    const primaryCallback = jest.fn();
    const secondaryCallback = jest.fn();
    render(
      <Alert
        primaryAction={{ content: 'Primary Button', onClick: primaryCallback }}
        secondaryAction={{
          content: 'Secondary Button',
          onClick: secondaryCallback,
        }}
      />
    );

    await user.click(screen.getByRole('button', { name: 'Primary Button' }));
    expect(primaryCallback).toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: 'Primary Button' }));
    expect(primaryCallback).toHaveBeenCalled();
  });

  test('dismiss button', async () => {
    const user = userEvent.setup();
    const onDismiss = jest.fn();
    render(
      <Alert dismissible onDismiss={onDismiss}>
        Message
      </Alert>
    );
    expect(screen.getByText('Message')).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(screen.queryByText('Message')).not.toBeInTheDocument();
    expect(onDismiss).toHaveBeenCalled();
  });
});

describe('Banner', () => {
  test.each`
    kind         | background
    ${undefined} | ${theme.colors.interactive.tonal.danger[2].background}
    ${'neutral'} | ${theme.colors.levels.elevated}
    ${'primary'} | ${theme.colors.interactive.tonal.primary[2].background}
    ${'danger'}  | ${theme.colors.interactive.tonal.danger[2].background}
    ${'warning'} | ${theme.colors.interactive.tonal.alert[2].background}
    ${'info'}    | ${theme.colors.interactive.tonal.informational[2].background}
    ${'success'} | ${theme.colors.interactive.tonal.success[2].background}
  `('renders appropriate background for kind $kind', ({ kind, background }) => {
    const { container } = render(<Banner kind={kind} />);
    expect(container.firstChild).toHaveStyle({ background });
  });

  test('action buttons', async () => {
    const user = userEvent.setup();
    const primaryCallback = jest.fn();
    const secondaryCallback = jest.fn();
    render(
      <Banner
        primaryAction={{ content: 'Primary Button', onClick: primaryCallback }}
        secondaryAction={{
          content: 'Secondary Button',
          onClick: secondaryCallback,
        }}
      />
    );

    await user.click(screen.getByRole('button', { name: 'Primary Button' }));
    expect(primaryCallback).toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: 'Primary Button' }));
    expect(primaryCallback).toHaveBeenCalled();
  });

  test('dismiss button', async () => {
    const user = userEvent.setup();
    const onDismiss = jest.fn();
    render(
      <Banner dismissible onDismiss={onDismiss}>
        Message
      </Banner>
    );
    expect(screen.getByText('Message')).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(screen.queryByText('Message')).not.toBeInTheDocument();
    expect(onDismiss).toHaveBeenCalled();
  });
});
