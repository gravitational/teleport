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
import { MemoryRouter } from 'react-router';

import { AwsStatusDetails } from './AwsOidcStatus';

export default {
  title: 'Teleport/Integrations/Status/Aws',
};

// External audit storage CTA
// is this a individual flag?
// does it fall under identity?
// is it enterprise only?
export const WithoutCta = () => (
  <MemoryRouter>
    <AwsStatusDetails />
  </MemoryRouter>
);

// External audit storage CTA
// is this a individual flag?
// does it fall under identity?
// is it enterprise only?
export const WithOpenSourceCta = () => (
  <MemoryRouter>
    <AwsStatusDetails />
  </MemoryRouter>
);

export const FailedFetching = () => (
  <MemoryRouter>
    <AwsStatusDetails />
  </MemoryRouter>
);

export const Loading = () => (
  <MemoryRouter>
    <AwsStatusDetails />
  </MemoryRouter>
);

// Can the integration be in a failed status??
// What will the status page look like
