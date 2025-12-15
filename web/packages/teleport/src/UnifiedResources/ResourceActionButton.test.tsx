/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { SamlAppActionProvider } from 'teleport/SamlApplications/useSamlAppActions';
import { App } from 'teleport/services/apps';

import { ResourceActionButton } from './ResourceActionButton';

test('saml app: empty launch url should display <a> link with href value pointed to samlAppSsoUrl', () => {
  render(
    <SamlAppActionProvider>
      <ResourceActionButton resource={resource} />
    </SamlAppActionProvider>
  );

  expect(screen.getByRole('link', { name: /Log In/i })).toHaveAttribute(
    'href',
    resource.samlAppSsoUrl
  );
});

test('saml app: launch url with one item should display <a> link with href value pointed to first launch url item', () => {
  const samlApp = resource;
  samlApp.samlAppLaunchUrls = [{ url: 'https://example.com' }];
  render(
    <MemoryRouter>
      <SamlAppActionProvider>
        <ResourceActionButton resource={samlApp} />
      </SamlAppActionProvider>
    </MemoryRouter>
  );

  expect(screen.getByRole('link', { name: /Log In/i })).toHaveAttribute(
    'href',
    samlApp.samlAppLaunchUrls[0].url
  );
});

test('saml app: multiple launch urls should display menu login button', async () => {
  const samlApp = resource;
  samlApp.samlAppLaunchUrls = [
    { url: 'https://example.com' },
    { url: 'https://2.example.com' },
  ];
  render(
    <MemoryRouter>
      <SamlAppActionProvider>
        <ResourceActionButton resource={samlApp} />
      </SamlAppActionProvider>
    </MemoryRouter>
  );

  fireEvent.click(screen.getByRole('button', { name: /Log In/i }));
  await waitFor(() => {
    expect(
      screen.getByText(samlApp.samlAppLaunchUrls[0].url)
    ).toBeInTheDocument();
  });
  expect(
    screen.getByText(samlApp.samlAppLaunchUrls[1].url)
  ).toBeInTheDocument();
});

const resource: App = {
  kind: 'app',
  name: 'saml_app',
  uri: '',
  publicAddr: '',
  description: 'SAML Application',
  awsConsole: false,
  labels: [],
  clusterId: 'one',
  fqdn: '',
  samlApp: true,
  samlAppSsoUrl: 'https://sp.example.com',
  id: 'saml_app.teleport.com',
  launchUrl: '',
  awsRoles: [],
  userGroups: [],
};
