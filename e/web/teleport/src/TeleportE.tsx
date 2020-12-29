import React from 'react';
import { Route, Switch } from 'teleport/components/Router';
import Console from 'teleport/Console';
import Player from 'teleport/Player';
import Teleport, { Props } from 'teleport/Teleport';
import cfg from 'teleport/config';
import WaitingRoom from 'e-teleport/WaitingRoom';
import { LicenseEnforcer } from './License';
import Main from './Main';

const TeleportE: React.FC<Props> = ({ history, ctx }) => {
  return (
    <Teleport history={history} ctx={ctx}>
      <WaitingRoom>
        <LicenseEnforcer />
        <Switch>
          <Route path={cfg.routes.console} component={Console} />
          <Route path={cfg.routes.player} component={Player} />
          <Route path={cfg.routes.root} component={Main} />
        </Switch>
      </WaitingRoom>
    </Teleport>
  );
};

export default TeleportE;
