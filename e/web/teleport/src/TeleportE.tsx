import React from 'react';
import { Route, Switch } from 'teleport/components/Router';
import Console from 'teleport/Console';
import Player from 'teleport/Player';
import Teleport from 'teleport/Teleport';
import cfg from 'teleport/config';
import AccessStrategy from 'teleport/AccessStrategy';
import { LicenseEnforcer } from './License';
import Main from './Main';

export default function TeleportE({ history }) {
  return (
    <Teleport history={history}>
      <AccessStrategy>
        <LicenseEnforcer />
        <Switch>
          <Route path={cfg.routes.console} component={Console} />
          <Route path={cfg.routes.player} component={Player} />
          <Route path={cfg.routes.root} component={Main} />
        </Switch>
      </AccessStrategy>
    </Teleport>
  );
}
