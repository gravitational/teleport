/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
