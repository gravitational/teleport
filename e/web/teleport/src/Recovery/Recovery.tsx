import React from 'react';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'e-teleport/config';
import LogoHero from 'teleport/components/LogoHero';
import RecoveryStart from './RecoveryStart';
import RecoveryFlow from './RecoveryFlow';

export default function UserRecovery() {
  return (
    <>
      <LogoHero />
      <Switch>
        <Route exact path={cfg.routes.recoveryForgotPassword}>
          <RecoveryStart recoveryType="password" />
        </Route>
        <Route exact path={cfg.routes.recoveryForgotDevice}>
          <RecoveryStart recoveryType="device" />
        </Route>
        <Route path={cfg.routes.recoverySteps} component={RecoveryFlow} />
      </Switch>
    </>
  );
}
