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

import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { Redirect, Route, Switch, useLocation, useParams } from './Router';

function renderWithRouter(
  initialEntry: string | { pathname: string; state?: unknown },
  element: ReactNode
) {
  render(
    <MemoryRouter initialEntries={[initialEntry]}>{element}</MemoryRouter>
  );
}

describe('Switch', () => {
  test('supports nested absolute child routes by stripping the parent prefix', () => {
    renderWithRouter(
      '/web/cluster/root/apps/details',
      <Switch>
        <Route path="/web/cluster/:clusterId/apps">
          <Switch>
            <Route
              exact
              path="/web/cluster/:clusterId/apps"
              element={<div>apps home</div>}
            />
            <Route
              path="/web/cluster/:clusterId/apps/details"
              element={<div>apps details</div>}
            />
          </Switch>
        </Route>
      </Switch>
    );

    expect(screen.getByText('apps details')).toBeInTheDocument();
  });

  test('keeps exact nested absolute child routes from matching subpaths', () => {
    renderWithRouter(
      '/web/account/invalid-path',
      <Switch>
        <Route path="/web/account">
          <Switch>
            <Route
              exact
              path="/web/account"
              element={<div>account home</div>}
            />
          </Switch>
        </Route>
      </Switch>
    );

    expect(screen.queryByText('account home')).not.toBeInTheDocument();
    expect(
      screen.getByText('The requested path could not be found.')
    ).toBeInTheDocument();
  });

  test('adds a default 404 route when a wildcard route is not provided', () => {
    renderWithRouter(
      '/missing',
      <Switch>
        <Route path="/known" element={<div>known</div>} />
      </Switch>
    );

    expect(
      screen.getByText('The requested path could not be found.')
    ).toBeInTheDocument();
  });

  test('does not add a default 404 route when a wildcard route is provided', () => {
    renderWithRouter(
      '/missing',
      <Switch>
        <Route path="/known" element={<div>known</div>} />
        <Route element={<div>custom wildcard</div>} />
      </Switch>
    );

    expect(screen.getByText('custom wildcard')).toBeInTheDocument();
    expect(
      screen.queryByText('The requested path could not be found.')
    ).not.toBeInTheDocument();
  });

  test('uses router hooks in route elements', () => {
    function Probe() {
      const location = useLocation();
      const params = useParams<{ clusterId: string }>();
      const state = location.state as { integrationName: string };

      return <div>{`${state.integrationName}:${params.clusterId}`}</div>;
    }

    renderWithRouter(
      {
        pathname: '/cluster/root',
        state: { integrationName: 'ra-integration' },
      },
      <Switch>
        <Route path="/cluster/:clusterId" element={<Probe />} />
      </Switch>
    );

    expect(screen.getByText('ra-integration:root')).toBeInTheDocument();
  });

  test('redirects using route element', () => {
    renderWithRouter(
      '/bots/new',
      <Switch>
        <Route path="/bots/new/github" element={<div>bot flow</div>} />
        <Route
          path="/bots/new/:type?"
          element={<Redirect to="/integrations/new?tags=bot" />}
        />
        <Route path="/integrations/new" element={<div>integrations</div>} />
      </Switch>
    );

    expect(screen.getByText('integrations')).toBeInTheDocument();
  });
});
