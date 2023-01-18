import React from 'react';
import { Box } from 'design';

import { Route, Switch, NavLink, Redirect } from 'teleport/components/Router';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
  TabItem,
} from 'teleport/components/Layout';
import ChangePassword from 'teleport/Account/ChangePassword';
import ManageDevices from 'teleport/Account/ManageDevices';

import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'e-teleport/config';

import Recovery from './Recovery';

export default function Container() {
  const ctx = useTeleportE();
  return <Account isSso={ctx.storeUser.isSso()} />;
}

export function Account({ isSso }: Props) {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" mb={0}>
        <FeatureHeaderTitle>
          {!isSso && (
            <TabItem as={NavLink} to={cfg.oss.routes.accountPassword}>
              Password
            </TabItem>
          )}
          <TabItem as={NavLink} to={cfg.oss.routes.accountMfaDevices}>
            Two-Factor Devices
          </TabItem>
          {cfg.oss.recoveryCodesEnabled && (
            <TabItem as={NavLink} to={cfg.routes.accountRecovery}>
              Recovery
            </TabItem>
          )}
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Box>
        <Switch>
          {!isSso && (
            <Route
              path={cfg.oss.routes.accountPassword}
              component={ChangePassword}
            />
          )}
          <Route
            path={cfg.oss.routes.accountMfaDevices}
            component={ManageDevices}
          />
          {cfg.oss.recoveryCodesEnabled && (
            <Route path={cfg.routes.accountRecovery} component={Recovery} />
          )}
          <Redirect
            to={
              isSso
                ? cfg.oss.routes.accountMfaDevices
                : cfg.oss.routes.accountPassword
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
