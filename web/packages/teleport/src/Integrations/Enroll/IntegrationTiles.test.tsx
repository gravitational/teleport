/**
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

import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';

import { IntegrationTiles } from './IntegrationTiles';

test('render', async () => {
  render(
    <MemoryRouter>
      <IntegrationTiles />
    </MemoryRouter>
  );

  expect(screen.getByText(/AWS OIDC Identity Provider/i)).toBeInTheDocument();
  expect(screen.queryByText(/no permission/i)).not.toBeInTheDocument();
  expect(screen.getAllByTestId('res-icon-aws')).toHaveLength(2);
  expect(screen.getAllByRole('link')).toHaveLength(2);

  const tile = screen.getByTestId('tile-aws-oidc');
  expect(tile).toBeEnabled();
  expect(tile.getAttribute('href')).toBeTruthy();
});

test('render disabled', async () => {
  render(
    <MemoryRouter>
      <IntegrationTiles
        hasIntegrationAccess={false}
        hasExternalAuditStorage={false}
      />
    </MemoryRouter>
  );

  expect(screen.queryByRole('link')).not.toBeInTheDocument();
  expect(
    screen.queryByText(/request additional permissions/i)
  ).not.toBeInTheDocument();

  const tile = screen.getByTestId('tile-aws-oidc');
  expect(tile).not.toHaveAttribute('href');

  // The element has disabled attribute, but it's in the format `disabled=""`
  // so "toBeDisabled" interprets it as false.
  // eslint-disable-next-line jest-dom/prefer-enabled-disabled
  expect(tile).toHaveAttribute('disabled');

  // Disabled states have badges on them. Test it renders on hover.
  const badge = screen.getByText(/lacking permission/i);
  await userEvent.hover(badge);
  expect(
    screen.getByText(/request additional permissions/i)
  ).toBeInTheDocument();
});

test('dont render External Audit Storage for enterprise unless it is cloud', async () => {
  cfg.isEnterprise = true;
  cfg.isCloud = false;

  render(
    <MemoryRouter>
      <IntegrationTiles />
    </MemoryRouter>
  );

  expect(
    screen.queryByText(/AWS External Audit Storage/i)
  ).not.toBeInTheDocument();
});
