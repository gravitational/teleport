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

import React from 'react';
import { Box } from 'design';

import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';
import { Route, Switch, NavLink, Redirect } from 'teleport/components/Router';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
  TabItem,
} from 'teleport/components/Layout';

import ChangePassword from './ChangePassword';
import ManageDevices from './ManageDevices';

export default function Container() {
  const ctx = useTeleport();
  return <Account isSso={ctx.storeUser.isSso()} />;
}

export function Account({ isSso }: Props) {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" mb={0}>
        <FeatureHeaderTitle>
          {!isSso && (
            <TabItem as={NavLink} to={cfg.routes.accountPassword}>
              Password
            </TabItem>
          )}
          <TabItem as={NavLink} to={cfg.routes.accountMfaDevices}>
            Two-Factor Devices
          </TabItem>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Box>
        <Switch>
          {!isSso && (
            <Route
              path={cfg.routes.accountPassword}
              component={ChangePassword}
            />
          )}
          <Route
            path={cfg.routes.accountMfaDevices}
            component={ManageDevices}
          />
          <Redirect
            to={
              isSso ? cfg.routes.accountMfaDevices : cfg.routes.accountPassword
            }
          />
        </Switch>
      </Box>
    </FeatureBox>
  );
}

type Props = {
  isSso: boolean;
};
