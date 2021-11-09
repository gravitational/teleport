import React from 'react';
import { Box } from 'design';
import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import { Route, Switch, NavLink, Redirect } from 'teleport/components/Router';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
  TabItem,
} from 'teleport/components/Layout';
import ChangePassword from 'teleport/Account/ChangePassword';
import ManageDevices from 'teleport/Account/ManageDevices';
import Recovery from './Recovery';

export default function Container() {
  const ctx = useTeleportE();
  return <Account isSso={ctx.storeUser.isSso()} />;
}

export function Account({ isSso }: Props) {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          {!isSso && (
            <TabItem as={NavLink} to={cfg.oss.routes.accountPassword}>
              Password
            </TabItem>
          )}
          <TabItem as={NavLink} to={cfg.oss.routes.accountMfaDevices}>
            Two-Factor Devices
          </TabItem>
          {cfg.oss.isCloud && (
            <TabItem as={NavLink} to={cfg.routes.accountRecovery}>
              Recovery
            </TabItem>
          )}
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Box mt={3}>
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
          {cfg.oss.isCloud && (
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
