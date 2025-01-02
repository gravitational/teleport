import React from 'react';

import cfg from 'e-teleport/config';
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

const TeleportE: React.FC<Props> = ({ history, ctx }) => {
  return (
    <Teleport
      history={history}
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
      component={Login}
    />,
    <Route
      key="ent-4"
      title="Recovery"
      path={cfg.routes.recovery}
      component={Recovery}
    />,
    <Route
      key="invite"
      title="Invite"
      path={ossConfig.routes.userInvite}
      render={() => <Welcome NewCredentials={ENewCredentials} />}
    />,
    <Route
      key="password-reset"
      title="Password Reset"
      path={ossConfig.routes.userReset}
      render={() => <Welcome NewCredentials={ENewCredentials} />}
    />,
    ...getSharedPublicRoutes(),
  ];
}

function privateERoutes() {
  return (
    <WaitingRoom>
      <Switch>
        <Route path={cfg.routes.samlIdPLogin} component={SAMLIdPLogin} />
        {getSharedPrivateRoutes()}
        <Route path={ossConfig.routes.root} component={Main} />
      </Switch>
    </WaitingRoom>
  );
}

export default TeleportE;
