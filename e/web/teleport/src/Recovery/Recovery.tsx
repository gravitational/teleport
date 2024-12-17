import { Route, Switch } from 'teleport/components/Router';

import { LogoHero } from 'teleport/components/LogoHero';

import cfg from 'e-teleport/config';

import RecoveryStart from './RecoveryStart';
import RecoveryFlow from './RecoveryFlow';

export function Recovery() {
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
