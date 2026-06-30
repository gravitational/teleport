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

import { MemoryRouter } from 'react-router';

import { CurrentPath, render, screen, userEvent } from 'design/utils/testing';

import { Alert, Banner } from '.';

describe('Alert', () => {
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

  test('action buttons as external links', async () => {
    render(
      <Banner
        primaryAction={{
          content: 'Primary Link',
          href: 'https://goteleport.com/1',
        }}
        secondaryAction={{
          content: 'Secondary Link',
          href: 'https://goteleport.com/2',
        }}
      />
    );

    expect(screen.getByRole('link', { name: 'Primary Link' })).toHaveAttribute(
      'href',
      'https://goteleport.com/1'
    );
    expect(screen.getByRole('link', { name: 'Primary Link' })).toHaveAttribute(
      'target',
      '_blank'
    );
    expect(
      screen.getByRole('link', { name: 'Secondary Link' })
    ).toHaveAttribute('href', 'https://goteleport.com/2');
    expect(
      screen.getByRole('link', { name: 'Secondary Link' })
    ).toHaveAttribute('target', '_blank');
  });

  test('action buttons as internal links', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter initialEntries={['/']}>
        <Banner
          primaryAction={{
            content: 'Primary Link',
            linkTo: 'primary-route',
          }}
          secondaryAction={{
            content: 'Secondary Link',
            linkTo: 'secondary-route',
          }}
        />
        <CurrentPath />
      </MemoryRouter>
    );

    expect(screen.getByTestId('current-path')).toHaveTextContent('/');

    expect(screen.getByRole('link', { name: 'Primary Link' })).toHaveAttribute(
      'href',
      '/primary-route'
    );
    expect(
      screen.getByRole('link', { name: 'Primary Link' })
    ).not.toHaveAttribute('target');
    await user.click(screen.getByRole('link', { name: 'Primary Link' }));
    expect(screen.getByTestId('current-path')).toHaveTextContent(
      '/primary-route'
    );

    expect(
      screen.getByRole('link', { name: 'Secondary Link' })
    ).toHaveAttribute('href', '/secondary-route');
    expect(
      screen.getByRole('link', { name: 'Secondary Link' })
    ).not.toHaveAttribute('target');
    await user.click(screen.getByRole('link', { name: 'Secondary Link' }));
    expect(screen.getByTestId('current-path')).toHaveTextContent(
      '/secondary-route'
    );
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
