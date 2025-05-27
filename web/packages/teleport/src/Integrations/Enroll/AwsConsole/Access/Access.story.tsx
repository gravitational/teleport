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

import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { IamIntegration } from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/IamIntegration';

export default {
  title: 'Teleport/Integrations/Enroll/AwsConsole',
};

export const SetupIAM = () => (
  <MemoryRouter>
    <IamIntegration />
  </MemoryRouter>
);

export const ConfigureAccess_NoProfiles = () => (
  <MemoryRouter>
    <Access />
  </MemoryRouter>
);

export const ConfigureAccess_WithProfiles = () => (
  <MemoryRouter>
    <Access />
  </MemoryRouter>
);
