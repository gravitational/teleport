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

import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { AwsRolesAnywhereSetup } from 'teleport/Integrations/Enroll/AwsConsole/AwsRolesAnywhereSetup';
import { IntegrationKind } from 'teleport/services/integrations';

import { AwsOidc } from './AwsOidc';

export function getRoutesToEnrollIntegrations() {
  return [
    <Route
      key={IntegrationKind.AwsRa}
      path={cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa)}
      component={AwsRolesAnywhereSetup}
    />,
    <Route
      key={IntegrationKind.AwsOidc}
      exact
      path={cfg.getIntegrationEnrollRoute(IntegrationKind.AwsOidc)}
      component={AwsOidc}
    />,
  ];
}
