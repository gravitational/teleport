/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { SingleLogoutFailed } from './SingleLogoutFailed';

export default {
  title: 'Teleport/LogoutError',
};

export const FailedDefault = () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/msg/error/slo'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <SingleLogoutFailed />
    </Router>
  );
};

export const FailedOkta = () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/msg/error/slo?connectorName=Okta'],
    initialIndex: 0,
  });

  return (
    <Router history={history}>
      <SingleLogoutFailed />
    </Router>
  );
};
