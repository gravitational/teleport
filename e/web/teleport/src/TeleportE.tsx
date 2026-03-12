import React from 'react';

import cfg from 'e-teleport/config';
import { ViewSessionRecordingRouteE } from 'e-teleport/SessionRecordings/view/ViewSessionRecordingRouteE';
import { WaitingRoom } from 'e-teleport/WaitingRoom';
import { ENewCredentials } from 'e-teleport/Welcome/NewCredentials';
import { Route, Switch } from 'teleport/components/Router';
import ossConfig from 'teleport/config';
import Teleport, {
  getSharedPrivateRoutes,
  getSharedPublicRoutes,
  Props,
} from 'teleport/Teleport';
import { Welcome } from 'teleport/Welcome';

import { Login } from './Login';
import { Main } from './Main';
import { Recovery } from './Recovery';
import { SAMLIdPLogin } from './SAMLIdPLogin';

const TeleportE: React.FC<Props> = ({ ctx }) => {
  return (
    <Teleport
      ctx={ctx}
      renderPublicRoutes={publicERoutes}
      renderPrivateRoutes={privateERoutes}
    />
  );
};

function publicERoutes() {
  return [
    <Route
      key="ent-1"
      title="Login"
      path={cfg.oss.routes.login}
      element={<Login />}
    />,
    <Route
      key="ent-4"
      title="Recovery"
      path={cfg.routes.recovery}
      element={<Recovery />}
    />,
    <Route
      key="invite"
      title="Invite"
      path={ossConfig.routes.userInvite}
      element={<Welcome NewCredentials={ENewCredentials} />}
    />,
    <Route
      key="password-reset"
      title="Password Reset"
      path={ossConfig.routes.userReset}
      element={<Welcome NewCredentials={ENewCredentials} />}
    />,
    ...getSharedPublicRoutes(),
  ];
}

function privateERoutes() {
  return (
    <WaitingRoom>
      <Switch>
        <Route path={cfg.routes.samlIdPLogin} element={<SAMLIdPLogin />} />
        <Route
          path={cfg.oss.routes.player}
          element={<ViewSessionRecordingRouteE />}
        />
        {getSharedPrivateRoutes()}
        <Route path={ossConfig.routes.root} element={<Main />} />
      </Switch>
    </WaitingRoom>
  );
}

export default TeleportE;
