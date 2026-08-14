import cfg from 'e-teleport/config';
import { LogoHero } from 'teleport/components/LogoHero';
import { Route, Switch } from 'teleport/components/Router';

import RecoveryFlow from './RecoveryFlow';
import RecoveryStart from './RecoveryStart';

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
        <Route path={cfg.routes.recoverySteps} element={<RecoveryFlow />} />
      </Switch>
    </>
  );
}
